package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/brunogallotte/macsweep/internal/scan"
)

// Browser walks a scanned tree the way ncdu does, but showing the share each
// child takes of its parent so the eye lands on the culprit immediately.
type Browser struct {
	theme  *Theme
	cur    *scan.Node
	cursor int
	offset int
	width  int
	height int
	marked map[string]*scan.Node
	quit   bool

	// Standalone means the browser is the whole program.
	Standalone bool
}

func NewBrowser(t *Theme, root *scan.Node) *Browser {
	root.SortBySize()
	return &Browser{theme: t, cur: root, width: 100, height: 24, marked: map[string]*scan.Node{}}
}

func (b *Browser) Init() tea.Cmd { return nil }

func (b *Browser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		b.width, b.height = m.Width, m.Height

	case tea.KeyPressMsg:
		switch m.String() {
		case "ctrl+c", "q":
			b.quit = true
			return b, b.finish()
		case "enter":
			if len(b.marked) > 0 {
				return b, b.finish()
			}
			b.descend()
		case "right", "l":
			b.descend()
		case "left", "h", "backspace":
			b.ascend()
		case "up", "k":
			b.move(-1)
		case "down", "j":
			b.move(1)
		case "pgup":
			b.move(-b.page())
		case "pgdown":
			b.move(b.page())
		case "g":
			b.cursor, b.offset = 0, 0
		case "G":
			b.move(len(b.cur.Children))
		case "space", " ":
			b.mark()
		}
	}
	return b, nil
}

func (b *Browser) finish() tea.Cmd {
	if b.Standalone {
		return tea.Quit
	}
	confirmed := !b.quit
	return func() tea.Msg { return DoneMsg{Confirmed: confirmed} }
}

// RunBrowser shows the tree as a program of its own and returns what was
// marked for removal.
func RunBrowser(t *Theme, root *scan.Node) ([]Row, error) {
	b := NewBrowser(t, root)
	b.Standalone = true
	if _, err := tea.NewProgram(b).Run(); err != nil {
		return nil, err
	}
	return b.Chosen(), nil
}

func (b *Browser) page() int { return max(b.height-7, 1) }

func (b *Browser) move(d int) {
	b.cursor = min(max(b.cursor+d, 0), max(len(b.cur.Children)-1, 0))
	if b.cursor < b.offset {
		b.offset = b.cursor
	}
	if b.cursor >= b.offset+b.page() {
		b.offset = b.cursor - b.page() + 1
	}
}

func (b *Browser) selected() *scan.Node {
	if b.cursor < len(b.cur.Children) {
		return b.cur.Children[b.cursor]
	}
	return nil
}

func (b *Browser) descend() {
	n := b.selected()
	if n != nil && n.IsDir && len(n.Children) > 0 {
		b.cur, b.cursor, b.offset = n, 0, 0
	}
}

func (b *Browser) ascend() {
	if b.cur.Parent == nil {
		return
	}
	child := b.cur
	b.cur = b.cur.Parent
	b.cursor, b.offset = 0, 0
	for i, c := range b.cur.Children {
		if c == child {
			b.cursor = i
			break
		}
	}
	if b.cursor >= b.page() {
		b.offset = b.cursor - b.page() + 1
	}
}

func (b *Browser) mark() {
	n := b.selected()
	if n == nil {
		return
	}
	if _, ok := b.marked[n.Path]; ok {
		delete(b.marked, n.Path)
		return
	}
	b.marked[n.Path] = n
}

// Chosen returns what the user marked, as selection rows.
func (b *Browser) Chosen() []Row {
	if b.quit {
		return nil
	}
	var out []Row
	for path, n := range b.marked {
		out = append(out, Row{
			ID: path, Title: n.Name, Detail: path, Size: n.Size, Selected: true,
		})
	}
	return out
}

func (b *Browser) View() tea.View {
	t := b.theme
	var s strings.Builder

	s.WriteString(t.Title.Render(TruncatePath(b.cur.Path, b.width-20)))
	s.WriteString("  ")
	s.WriteString(t.Accent.Render(Size(b.cur.Size)))
	s.WriteString("\n")
	s.WriteString(t.Subtle.Render(fmt.Sprintf("%d itens, %d arquivos", len(b.cur.Children), b.cur.Files)))
	s.WriteString("\n\n")

	end := min(b.offset+b.page(), len(b.cur.Children))
	nameW := max(b.width-38, 16)

	for i := b.offset; i < end; i++ {
		n := b.cur.Children[i]

		cursor := "  "
		if i == b.cursor {
			cursor = t.Cursor.Render("▸ ")
		}
		mark := " "
		if _, ok := b.marked[n.Path]; ok {
			mark = t.Selected.Render("◉")
		}

		name := n.Name
		if n.IsDir {
			name += "/"
		}
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}

		flag := ""
		switch {
		case n.Restricted:
			flag = t.Subtle.Render(" [sistema]")
		case n.Dataless:
			flag = t.Subtle.Render(" [iCloud]")
		case n.Err != nil:
			flag = t.Warn.Render(" [sem permissao]")
		}

		fmt.Fprintf(&s, "%s%s %10s  %s  %s%s\n",
			cursor, mark,
			t.Size.Render(Size(n.Size)),
			t.SizeBar(n.Size, b.cur.Size, 14),
			name+strings.Repeat(" ", max(nameW-len([]rune(name)), 0)),
			flag,
		)
	}

	if len(b.cur.Children) == 0 {
		s.WriteString(t.Subtle.Render("  vazio\n"))
	}

	s.WriteString("\n  ")
	if len(b.marked) > 0 {
		var total int64
		for _, n := range b.marked {
			total += n.Size
		}
		s.WriteString(t.Accent.Render(fmt.Sprintf("%d marcados, %s  ", len(b.marked), Size(total))))
	}
	s.WriteString(t.Keys(
		"setas", "navegar",
		"espaco", "marcar",
		"enter", "remover marcados",
		"q", "sair",
	))
	s.WriteString("\n")

	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}
