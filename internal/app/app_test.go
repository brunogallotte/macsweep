package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/brunogallotte/macsweep/internal/modules"
	"github.com/brunogallotte/macsweep/internal/safety"
	"github.com/brunogallotte/macsweep/internal/ui"
)

func newTestModel(t *testing.T) *model {
	t.Helper()
	home := t.TempDir()
	return newModel(context.Background(), Options{
		Env: &modules.Env{
			Home:  home,
			Guard: safety.New(home, os.Getuid(), nil),
		},
		Theme:    ui.NewPlainTheme(),
		Home:     home,
		StateDir: t.TempDir(),
		// Every test runs in simulation, so nothing can be removed by a test.
		DryRun: true,
	})
}

func press(m *model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		if len(k) == 1 {
			msg = tea.KeyPressMsg{Code: rune(k[0]), Text: k}
		} else {
			switch k {
			case "down":
				msg = tea.KeyPressMsg{Code: tea.KeyDown}
			case "up":
				msg = tea.KeyPressMsg{Code: tea.KeyUp}
			case "enter":
				msg = tea.KeyPressMsg{Code: tea.KeyEnter}
			case "space":
				msg = tea.KeyPressMsg{Code: tea.KeySpace}
			default:
				panic("tecla nao mapeada: " + k)
			}
		}
		m.Update(msg)
	}
}

// runBatch executes a command, unwrapping tea.Batch, and feeds every message
// it produces back into the model.
func runBatch(m *model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			runBatch(m, c)
		}
	case nil:
	default:
		m.Update(msg)
	}
}

func TestMenuRenders(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := m.View().Content
	for _, want := range []string{"macsweep", "Aplicativos", "Caches", "Projetos", "Analisar disco", "Sair"} {
		if !strings.Contains(view, want) {
			t.Errorf("o menu nao mostrou %q", want)
		}
	}
	if !strings.Contains(view, "simulacao") {
		t.Error("o menu deveria avisar que esta em modo simulacao")
	}
}

// Emptying only shows up when there is something to empty, and it says how
// much, because it is the only action that really returns space.
func TestEmptyEntryAppearsOnlyWhenThereIsSomething(t *testing.T) {
	if got := buildEntries(0); hasTitle(got, "Esvaziar a Lixeira") {
		t.Error("nao deveria oferecer esvaziar com a Lixeira vazia")
	}
	got := buildEntries(5 << 30)
	if !hasTitle(got, "Esvaziar a Lixeira") {
		t.Fatal("deveria oferecer esvaziar quando ha itens")
	}
	for _, e := range got {
		if e.title == "Esvaziar a Lixeira" && !strings.Contains(e.hint, "5.0 GB") {
			t.Errorf("a dica deveria dizer quanto libera, disse %q", e.hint)
		}
	}
}

func hasTitle(entries []entry, title string) bool {
	for _, e := range entries {
		if e.title == title {
			return true
		}
	}
	return false
}

func TestMenuNavigation(t *testing.T) {
	m := newTestModel(t)
	press(m, "down", "down")
	if m.cursor != 2 {
		t.Fatalf("cursor deveria estar em 2, esta em %d", m.cursor)
	}
	press(m, "up")
	if m.cursor != 1 {
		t.Fatalf("cursor deveria estar em 1, esta em %d", m.cursor)
	}
	// It must not run off either end.
	press(m, "up", "up", "up")
	if m.cursor != 0 {
		t.Errorf("cursor passou do topo: %d", m.cursor)
	}
}

// The loop the user asked for: run something, see the result, land back on
// the menu instead of being dropped at the shell.
func TestFullCycleReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	dir := t.TempDir()
	path := filepath.Join(dir, "node_modules")
	os.MkdirAll(path, 0o755)

	m.kind = modules.KindProjects
	m.Update(scannedMsg{
		kind: modules.KindProjects,
		res: modules.Result{
			Title: "Artefatos", Subtitle: "1 item",
			Rows: []ui.Row{{ID: path, Title: "node_modules", Detail: path,
				Size: 1 << 30, Selected: true}},
		},
	})
	if m.phase != phaseSelect {
		t.Fatalf("depois do scan deveria estar em selecao, esta em %v", m.phase)
	}
	if !strings.Contains(m.View().Content, "node_modules") {
		t.Error("a tela de selecao nao mostrou o item")
	}

	// Confirm the selection and let the work command run (dry run).
	_, cmd := m.Update(ui.DoneMsg{Confirmed: true})
	if m.phase != phaseWorking {
		t.Fatalf("deveria estar trabalhando, esta em %v", m.phase)
	}
	// work() returns a batch: the frame ticker plus the actual job.
	runBatch(m, cmd)
	if m.phase != phaseResult {
		t.Fatalf("deveria estar no resultado, esta em %v", m.phase)
	}
	if !strings.Contains(m.View().Content, "simulacao") {
		t.Error("o resultado deveria dizer que foi simulacao")
	}

	// Any key goes back to the menu, which is the whole point.
	press(m, "x")
	if m.phase != phaseMenu {
		t.Fatalf("deveria ter voltado ao menu, esta em %v", m.phase)
	}
	if !strings.Contains(m.View().Content, "Aplicativos") {
		t.Error("o menu nao voltou")
	}
}

// Backing out of a selection returns to the menu and removes nothing.
func TestCancellingSelectionReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	m.kind = modules.KindClean
	m.Update(scannedMsg{kind: modules.KindClean, res: modules.Result{
		Rows: []ui.Row{{ID: "x", Title: "cache", Detail: "/tmp/x", Size: 1}},
	}})
	m.Update(ui.DoneMsg{Confirmed: false})

	if m.phase != phaseMenu {
		t.Fatalf("cancelar deveria voltar ao menu, foi para %v", m.phase)
	}
}

// A module that found nothing says so and does not open an empty list.
func TestEmptyScanGoesStraightToResult(t *testing.T) {
	m := newTestModel(t)
	m.Update(scannedMsg{kind: modules.KindClean, res: modules.Result{}})
	if m.phase != phaseResult {
		t.Fatalf("deveria ir direto ao resultado, foi para %v", m.phase)
	}
	if !strings.Contains(m.View().Content, "nada encontrado") {
		t.Error("deveria dizer que nao encontrou nada")
	}
}

// Uninstalling passes through a preview of every path before anything moves.
func TestAppsFlowShowsPreview(t *testing.T) {
	m := newTestModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.kind = modules.KindApps

	m.Update(scannedMsg{kind: modules.KindApps, res: modules.Result{
		Title:   "Aplicativos",
		Preview: true,
		Rows:    []ui.Row{{ID: "com.exemplo.app", Title: "Exemplo", Size: 1 << 30, Selected: true}},
		Expand: func(chosen []ui.Row) ([]ui.Row, []modules.Command) {
			return []ui.Row{
				{ID: "/Applications/Exemplo.app", Title: "aplicativo",
					Detail: "/Applications/Exemplo.app", Size: 1 << 30, Selected: true},
				{ID: "/cache", Title: "cache", Detail: "/cache", Size: 1 << 20, Selected: true},
			}, nil
		},
	}})

	m.Update(ui.DoneMsg{Confirmed: true})
	if m.phase != phasePreview {
		t.Fatalf("deveria mostrar a previa, esta em %v", m.phase)
	}
	view := m.View().Content
	if !strings.Contains(view, "Confira antes de remover") || !strings.Contains(view, "/cache") {
		t.Errorf("a previa nao lista os caminhos:\n%s", view)
	}
}
