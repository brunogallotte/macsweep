// Package app is macsweep as a program you stay inside.
//
// Running a subcommand does one thing and exits, which is right for scripts
// and wrong for a person cleaning a disk: they want to look at projects, then
// at caches, then empty the Trash, and see the disk fill back up as they go.
// This package is that loop. Every module reaches it through
// internal/modules, the same path the subcommands take, so the two can never
// disagree about what a module does.
package app

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/brunogallotte/macsweep/internal/modules"
	"github.com/brunogallotte/macsweep/internal/scan"
	"github.com/brunogallotte/macsweep/internal/trash"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// Options is how the CLI hands its flags over to the interactive app.
type Options struct {
	Env       *modules.Env
	Theme     *ui.Theme
	Home      string
	StateDir  string
	DryRun    bool
	Permanent bool
}

// Run opens the app and blocks until the user leaves.
func Run(ctx context.Context, opts Options) error {
	_, err := tea.NewProgram(newModel(ctx, opts)).Run()
	return err
}

type phase int

const (
	phaseMenu phase = iota
	phaseScanning
	phaseSelect
	phasePreview
	phaseBrowse
	phaseWorking
	phaseResult
)

// analyze is handled next to the modules rather than inside them, because it
// produces a tree to walk instead of a list to tick.
const kindAnalyze modules.Kind = "analyze"

type model struct {
	ctx  context.Context
	opts Options
	t    *ui.Theme

	phase         phase
	width, height int

	entries []entry
	cursor  int

	vol     scan.Volume
	inTrash int64

	kind    modules.Kind
	res     modules.Result
	sel     *ui.Selector
	browser *ui.Browser
	items   []ui.Row
	cmds    []modules.Command

	status  *statusBox
	frame   int
	started time.Time

	outcome ui.Outcome
	err     error
}

func newModel(ctx context.Context, opts Options) *model {
	m := &model{
		ctx:    ctx,
		opts:   opts,
		t:      opts.Theme,
		width:  100,
		height: 30,
		status: &statusBox{},
	}
	m.refresh()
	return m
}

// refresh re-reads the disk so the numbers on the menu are current every time
// the user comes back to it. Watching the free space move after a cleanup is
// most of the reward.
func (m *model) refresh() {
	m.vol, _ = scan.VolumeAt(m.opts.Home)
	if _, n, err := trash.PendingItems(m.opts.StateDir); err == nil {
		m.inTrash = n
	}
	m.entries = buildEntries(m.inTrash)
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
}

func (m *model) Init() tea.Cmd { return nil }

// --- messages ---

type scannedMsg struct {
	kind modules.Kind
	res  modules.Result
	tree *scan.Node
	err  error
}

type appliedMsg struct{ out ui.Outcome }

type frameMsg struct{}

func tick() tea.Cmd {
	// One frame every 80ms. The scanners write their progress into a shared
	// box and this reads it, instead of sending a message per file, which at
	// two million files would be two million calls to Update.
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return frameMsg{} })
}

type statusBox struct {
	mu   sync.Mutex
	text string
}

func (s *statusBox) set(t string) {
	s.mu.Lock()
	s.text = t
	s.mu.Unlock()
}

func (s *statusBox) get() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.text
}

// --- update ---

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.forwardSize(msg)
		return m, nil

	case frameMsg:
		m.frame++
		if m.phase == phaseScanning || m.phase == phaseWorking {
			return m, tick()
		}
		return m, nil

	case scannedMsg:
		return m.onScanned(msg)

	case appliedMsg:
		m.outcome = msg.out
		m.phase = phaseResult
		m.refresh()
		return m, nil

	case ui.DoneMsg:
		return m.onSelectionDone(msg)

	case tea.KeyPressMsg:
		return m.onKey(msg)
	}

	return m.forward(msg)
}

func (m *model) forwardSize(msg tea.WindowSizeMsg) {
	if m.sel != nil {
		m.sel.Update(msg)
	}
	if m.browser != nil {
		m.browser.Update(msg)
	}
}

// forward hands a message to whichever sub model owns the screen.
func (m *model) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseSelect, phasePreview:
		if m.sel != nil {
			_, cmd := m.sel.Update(msg)
			return m, cmd
		}
	case phaseBrowse:
		if m.browser != nil {
			_, cmd := m.browser.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseMenu:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			m.cursor = max(m.cursor-1, 0)
		case "down", "j":
			m.cursor = min(m.cursor+1, len(m.entries)-1)
		case "r":
			m.refresh()
		case "enter", "space", " ", "right", "l":
			return m.start(m.entries[m.cursor])
		}
		return m, nil

	case phaseScanning, phaseWorking:
		if s := msg.String(); s == "ctrl+c" || s == "esc" || s == "q" {
			// Leaving during a scan is safe: nothing has been touched yet.
			// Leaving during work is not offered, the phase check below keeps
			// the key inert there.
			if m.phase == phaseScanning {
				m.phase = phaseMenu
			}
		}
		return m, nil

	case phaseResult:
		m.phase = phaseMenu
		m.sel, m.browser, m.items, m.cmds = nil, nil, nil, nil
		return m, nil
	}

	return m.forward(msg)
}

func (m *model) onScanned(msg scannedMsg) (tea.Model, tea.Cmd) {
	m.err = msg.err
	if msg.err != nil {
		m.phase = phaseResult
		return m, nil
	}

	if msg.kind == kindAnalyze {
		m.browser = ui.NewBrowser(m.t, msg.tree)
		m.browser.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
		m.phase = phaseBrowse
		return m, nil
	}

	m.res = msg.res
	if m.res.Empty() {
		m.outcome = ui.Outcome{Command: string(msg.kind)}
		m.phase = phaseResult
		return m, nil
	}

	m.sel = ui.NewSelector(m.t, m.res.Title, m.res.Subtitle, m.res.Rows)
	m.sel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
	m.phase = phaseSelect
	return m, nil
}

func (m *model) onSelectionDone(msg ui.DoneMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case phaseBrowse:
		marked := m.browser.Chosen()
		if !msg.Confirmed || len(marked) == 0 {
			m.phase = phaseMenu
			return m, nil
		}
		return m.work(marked, nil)

	case phaseSelect:
		if !msg.Confirmed {
			m.phase = phaseMenu
			return m, nil
		}
		chosen := m.sel.Chosen()
		if len(chosen) == 0 {
			m.phase = phaseMenu
			return m, nil
		}

		items, cmds := chosen, []modules.Command(nil)
		if m.res.Expand != nil {
			items, cmds = m.res.Expand(chosen)
		}

		// Uninstalling shows every path before anything moves, because a tool
		// that hides what it touches is one people stop trusting.
		if m.res.Preview && len(items) > 0 {
			m.items, m.cmds = items, cmds
			m.sel = ui.NewSelector(m.t, "Confira antes de remover",
				sizeLine(items), ui.PreviewRows(items, m.opts.Home))
			m.sel.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
			m.phase = phasePreview
			return m, nil
		}
		return m.work(items, cmds)

	case phasePreview:
		if !msg.Confirmed {
			m.phase = phaseMenu
			return m, nil
		}
		return m.work(m.sel.Chosen(), m.cmds)
	}
	return m, nil
}

func (m *model) start(e entry) (tea.Model, tea.Cmd) {
	if e.quit {
		return m, tea.Quit
	}

	m.kind = e.kind
	m.err = nil
	m.status.set("preparando")
	m.started = time.Now()
	m.phase = phaseScanning

	env := *m.opts.Env
	env.Progress = m.status.set

	ctx := m.ctx
	kind := e.kind

	return m, tea.Batch(tick(), func() tea.Msg {
		if kind == kindAnalyze {
			env.Progress("escaneando " + m.opts.Home)
			res, err := scan.Walk(ctx, m.opts.Home, scan.Options{})
			if err != nil {
				return scannedMsg{kind: kind, err: err}
			}
			res.Root.SortBySize()
			return scannedMsg{kind: kind, tree: res.Root}
		}
		r, err := modules.Scan(ctx, kind, &env)
		return scannedMsg{kind: kind, res: r, err: err}
	})
}

func (m *model) work(items []ui.Row, cmds []modules.Command) (tea.Model, tea.Cmd) {
	m.phase = phaseWorking
	m.status.set("removendo")
	m.started = time.Now()

	opts := modules.ApplyOptions{
		Command:   string(m.kind),
		Home:      m.opts.Home,
		StateDir:  m.opts.StateDir,
		DryRun:    m.opts.DryRun,
		Permanent: m.opts.Permanent,
		Purge:     m.kind == modules.KindEmpty,
	}

	return m, tea.Batch(tick(), func() tea.Msg {
		return appliedMsg{out: modules.Apply(items, cmds, opts)}
	})
}

func sizeLine(rows []ui.Row) string {
	var n int64
	for _, r := range rows {
		n += r.Size
	}
	return ui.Size(n) + " em " + itoa(len(rows)) + " itens"
}
