package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Row is one selectable line. Every module maps its own domain type onto this
// so the selection experience is identical across the tool.
type Row struct {
	ID     string
	Title  string // the name the user recognises
	Detail string // the path, dimmed
	Note   string // how it comes back, or why it is here
	Size   int64
	Group  string

	// Locked rows are visible but cannot be chosen. Showing them, rather than
	// hiding them, is deliberate: a user who goes looking for something the
	// tool refused to touch deserves to see it and read why.
	Locked     bool
	LockReason string
	Selected   bool
}

type lineKind int

const (
	lineHeader lineKind = iota
	lineRow
)

type line struct {
	kind  lineKind
	text  string
	index int // into rows, for lineRow
}

// Selector is a windowed multi-select list. It renders only the visible
// slice, so it stays responsive with thousands of rows, and it keeps a
// running total of what is marked.
type Selector struct {
	theme    *Theme
	title    string
	subtitle string
	rows     []Row
	lines    []line
	cursor   int // index into lines, always on a lineRow
	offset   int
	width    int
	height   int

	confirmed bool
	cancelled bool

	// Standalone means this selector is the whole program, so finishing
	// quits. Embedded in a larger app it emits DoneMsg instead and the app
	// decides what comes next.
	Standalone bool
}

// DoneMsg is emitted when an embedded selector finishes.
type DoneMsg struct {
	Confirmed bool
}

// NewSelector builds the list. Rows arrive in the order they will be shown;
// grouping headers are derived from consecutive equal Group values.
func NewSelector(t *Theme, title, subtitle string, rows []Row) *Selector {
	s := &Selector{theme: t, title: title, subtitle: subtitle, rows: rows, width: 100, height: 24}
	s.rebuild()
	return s
}

func (s *Selector) rebuild() {
	s.lines = s.lines[:0]
	group := ""
	for i, r := range s.rows {
		if r.Group != "" && r.Group != group {
			group = r.Group
			s.lines = append(s.lines, line{kind: lineHeader, text: group})
		}
		s.lines = append(s.lines, line{kind: lineRow, index: i})
	}
	if s.cursor >= len(s.lines) {
		s.cursor = 0
	}
	if len(s.lines) > 0 && s.lines[s.cursor].kind != lineRow {
		s.moveCursor(1)
	}
}

func (s *Selector) Init() tea.Cmd { return nil }

func (s *Selector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = m.Width, m.Height
		return s, nil

	case tea.KeyPressMsg:
		switch m.String() {
		case "ctrl+c", "q", "esc":
			s.cancelled = true
			return s, s.finish()
		case "enter":
			s.confirmed = true
			return s, s.finish()
		case "up", "k":
			s.moveCursor(-1)
		case "down", "j":
			s.moveCursor(1)
		case "pgup":
			for i := 0; i < s.pageSize(); i++ {
				s.moveCursor(-1)
			}
		case "pgdown":
			for i := 0; i < s.pageSize(); i++ {
				s.moveCursor(1)
			}
		case "home", "g":
			s.cursor = 0
			s.offset = 0
			if len(s.lines) > 0 && s.lines[0].kind != lineRow {
				s.moveCursor(1)
			}
		case "end", "G":
			s.cursor = len(s.lines) - 1
			s.moveCursor(0)
		case "space", " ":
			s.toggle()
		case "a":
			s.setAll(true)
		case "n":
			s.setAll(false)
		case "i":
			s.invert()
		}
	}
	return s, nil
}

func (s *Selector) finish() tea.Cmd {
	if s.Standalone {
		return tea.Quit
	}
	confirmed := s.confirmed
	return func() tea.Msg { return DoneMsg{Confirmed: confirmed} }
}

// RunSelector shows the list as a program of its own and returns what was
// marked. An empty result means the user backed out.
func RunSelector(t *Theme, title, subtitle string, rows []Row) ([]Row, error) {
	s := NewSelector(t, title, subtitle, rows)
	s.Standalone = true
	if _, err := tea.NewProgram(s).Run(); err != nil {
		return nil, err
	}
	return s.Chosen(), nil
}

func (s *Selector) moveCursor(delta int) {
	if len(s.lines) == 0 {
		return
	}
	next := s.cursor + delta
	for next >= 0 && next < len(s.lines) && s.lines[next].kind != lineRow {
		next += sign(delta)
		if delta == 0 {
			next--
		}
	}
	if next < 0 || next >= len(s.lines) {
		return
	}
	s.cursor = next
	s.scrollIntoView()
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}

func (s *Selector) pageSize() int { return max(s.height-6, 1) }

func (s *Selector) scrollIntoView() {
	size := s.pageSize()
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+size {
		s.offset = s.cursor - size + 1
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

func (s *Selector) toggle() {
	if len(s.lines) == 0 {
		return
	}
	r := &s.rows[s.lines[s.cursor].index]
	if r.Locked {
		return
	}
	r.Selected = !r.Selected
}

func (s *Selector) setAll(v bool) {
	for i := range s.rows {
		if !s.rows[i].Locked {
			s.rows[i].Selected = v
		}
	}
}

func (s *Selector) invert() {
	for i := range s.rows {
		if !s.rows[i].Locked {
			s.rows[i].Selected = !s.rows[i].Selected
		}
	}
}

// Chosen returns the marked rows.
func (s *Selector) Chosen() []Row {
	if s.cancelled {
		return nil
	}
	var out []Row
	for _, r := range s.rows {
		if r.Selected && !r.Locked {
			out = append(out, r)
		}
	}
	return out
}

func (s *Selector) Cancelled() bool { return s.cancelled || !s.confirmed }

func (s *Selector) totals() (count int, size int64) {
	for _, r := range s.rows {
		if r.Selected && !r.Locked {
			count++
			size += r.Size
		}
	}
	return
}

func (s *Selector) View() tea.View {
	t := s.theme
	var b strings.Builder

	b.WriteString(t.Title.Render(s.title))
	if s.subtitle != "" {
		b.WriteString("  ")
		b.WriteString(t.Subtle.Render(s.subtitle))
	}
	b.WriteString("\n\n")

	// Column widths: size is fixed, the name takes what is left after the
	// note, and the path fills whatever remains under the name.
	sizeW := 10
	noteW := 24
	nameW := max(s.width-sizeW-noteW-8, 20)

	size := s.pageSize()
	end := min(s.offset+size, len(s.lines))

	var maxSize int64
	for _, r := range s.rows {
		if r.Size > maxSize {
			maxSize = r.Size
		}
	}

	for i := s.offset; i < end; i++ {
		ln := s.lines[i]
		if ln.kind == lineHeader {
			b.WriteString(t.Header.Render("  " + ln.text))
			b.WriteString("\n")
			continue
		}

		r := s.rows[ln.index]
		cursor := "  "
		if i == s.cursor {
			cursor = t.Cursor.Render("▸ ")
		}

		mark := "◯"
		markStyle := t.Subtle
		switch {
		case r.Locked:
			mark = "✕"
			markStyle = t.Danger
		case r.Selected:
			mark = "◉"
			markStyle = t.Selected
		}

		name := r.Title
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		nameStyled := name
		if r.Locked {
			nameStyled = t.Locked.Render(name)
		}

		note := r.Note
		if r.Locked && r.LockReason != "" {
			note = r.LockReason
		}
		if len(note) > noteW {
			note = note[:noteW-1] + "…"
		}

		fmt.Fprintf(&b, "%s%s %s%s %s  %s\n",
			cursor,
			markStyle.Render(mark),
			nameStyled,
			strings.Repeat(" ", max(nameW-len(name), 0)),
			t.Size.Render(fmt.Sprintf("%*s", sizeW, Size(r.Size))),
			t.Subtle.Render(note),
		)

		// The focused row expands to show its full path and its share of the
		// largest item, which is where the drill-down detail lives without
		// needing a second pane.
		if i == s.cursor && r.Detail != "" {
			fmt.Fprintf(&b, "    %s  %s\n",
				t.SizeBar(r.Size, maxSize, 16),
				t.Subtle.Render(TruncatePath(r.Detail, s.width-24)),
			)
		}
	}

	if len(s.lines) == 0 {
		b.WriteString(t.Subtle.Render("  nada encontrado\n"))
	}

	count, total := s.totals()
	b.WriteString("\n")
	b.WriteString(t.Footer.Render(fmt.Sprintf("  %d de %d marcados", count, len(s.rows))))
	b.WriteString(t.Accent.Render("  ·  " + Size(total)))
	b.WriteString("\n  ")
	b.WriteString(t.Keys(
		"espaco", "marcar",
		"a", "todos",
		"i", "inverter",
		"enter", "confirmar",
		"q", "sair",
	))
	b.WriteString("\n")

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// PreviewRows rewrites rows so the path is what the eye lands on.
//
// In a normal list the name is what matters and only the focused row reveals
// its path. A confirmation screen is the opposite: the promise is that every
// path is visible before anything moves, so the path becomes the title and
// the kind moves to the side.
func PreviewRows(rows []Row, home string) []Row {
	out := make([]Row, len(rows))
	for i, r := range rows {
		r.Note = r.Title
		r.Title = ShortenHome(r.Detail, home)
		out[i] = r
	}
	return out
}

// ShortenHome replaces the home directory with ~, the way a person writes it.
func ShortenHome(path, home string) string {
	if home != "" && len(path) > len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}
