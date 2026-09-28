package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/brunogallotte/macsweep/internal/modules"
	"github.com/brunogallotte/macsweep/internal/ui"
)

type entry struct {
	kind  modules.Kind
	title string
	hint  string
	quit  bool
}

func buildEntries(inTrash int64) []entry {
	out := []entry{
		{kind: modules.KindApps, title: "Aplicativos",
			hint: "desinstala programas, do mais pesado ao mais leve"},
		{kind: modules.KindClean, title: "Caches",
			hint: "ferramentas de dev, apps e sistema"},
		{kind: modules.KindProjects, title: "Projetos",
			hint: "artefatos de build regeneraveis nos repositorios"},
		{kind: kindAnalyze, title: "Analisar disco",
			hint: "mapa navegavel de onde foram os GB"},
	}

	// Emptying only appears when there is something to empty, and it says how
	// much, because it is the only action that really returns space.
	if inTrash > 0 {
		out = append(out, entry{
			kind:  modules.KindEmpty,
			title: "Esvaziar a Lixeira",
			hint:  "libera " + ui.Size(inTrash) + " de verdade",
		})
	}
	return append(out, entry{title: "Sair", quit: true})
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *model) View() tea.View {
	var content string
	switch m.phase {
	case phaseMenu:
		content = m.viewMenu()
	case phaseScanning:
		content = m.viewBusy("escaneando")
	case phaseWorking:
		content = m.viewBusy("removendo")
	case phaseResult:
		content = m.viewResult()
	case phaseSelect, phasePreview:
		return m.sel.View()
	case phaseBrowse:
		return m.browser.View()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *model) viewMenu() string {
	t := m.t
	var b strings.Builder

	used := m.vol.Total - m.vol.Free
	width := min(max(m.width-6, 30), 66)

	b.WriteString("\n  " + t.Title.Render("macsweep") + "\n")
	b.WriteString("  " + t.Subtle.Render(strings.Repeat("─", width)) + "\n")
	fmt.Fprintf(&b, "  %s livres de %s\n",
		t.Size.Render(ui.Size(m.vol.Free)), t.Subtle.Render(ui.Size(m.vol.Total)))
	b.WriteString("  " + t.SizeBar(used, m.vol.Total, width) + "\n")

	if m.inTrash > 0 {
		fmt.Fprintf(&b, "  %s %s\n", t.Warn.Render("!"),
			t.Subtle.Render(ui.Size(m.inTrash)+" na Lixeira esperando para ser esvaziado"))
	}
	if m.opts.DryRun {
		fmt.Fprintf(&b, "  %s %s\n", t.Warn.Render("!"),
			t.Subtle.Render("modo simulacao: nada sera removido"))
	}
	b.WriteString("\n")

	for i, e := range m.entries {
		cursor := "  "
		title := e.title
		if i == m.cursor {
			cursor = t.Cursor.Render("▸ ")
			title = t.Accent.Render(e.title)
		}
		fmt.Fprintf(&b, "  %s%-28s %s\n",
			cursor, title+strings.Repeat(" ", max(18-len(e.title), 0)), t.Subtle.Render(e.hint))
	}

	b.WriteString("\n  ")
	b.WriteString(t.Keys("↑↓", "navegar", "enter", "abrir", "r", "atualizar", "q", "sair"))
	b.WriteString("\n")
	return b.String()
}

func (m *model) viewBusy(verb string) string {
	t := m.t
	var b strings.Builder

	b.WriteString("\n  " + t.Title.Render("macsweep") + "\n\n")
	fmt.Fprintf(&b, "  %s %s\n",
		t.Accent.Render(spinner[m.frame%len(spinner)]),
		t.Subtle.Render(m.status.get()))
	fmt.Fprintf(&b, "\n  %s\n",
		t.Subtle.Render(verb+" ha "+elapsed(m.started)))

	if m.phase == phaseScanning {
		b.WriteString("\n  " + t.Subtle.Render("nada foi tocado ainda, esc volta ao menu") + "\n")
	}
	return b.String()
}

func (m *model) viewResult() string {
	t := m.t
	var b strings.Builder

	b.WriteString("\n  " + t.Title.Render("macsweep") + "\n")

	switch {
	case m.err != nil:
		fmt.Fprintf(&b, "\n  %s %s\n", t.Danger.Render("erro:"), m.err)
	case m.outcome.Items == 0 && m.outcome.Failed == 0:
		b.WriteString("\n  " + t.Subtle.Render("nada encontrado para remover") + "\n")
	default:
		b.WriteString(t.Render(m.outcome))
	}

	b.WriteString("\n  " + t.Keys("qualquer tecla", "voltar ao menu") + "\n")
	return b.String()
}

func elapsed(start time.Time) string {
	d := time.Since(start)
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func itoa(n int) string { return strconv.Itoa(n) }
