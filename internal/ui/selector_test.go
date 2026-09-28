package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(s string) tea.KeyPressMsg {
	if len(s) == 1 {
		return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
	}
	switch s {
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	panic("tecla nao mapeada no teste: " + s)
}

func rows() []Row {
	return []Row{
		{ID: "a", Title: "node_modules", Detail: "/p/a/node_modules", Size: 3 << 30, Group: "projeto a"},
		{ID: "b", Title: ".next", Detail: "/p/a/.next", Size: 2 << 30, Group: "projeto a"},
		{ID: "c", Title: "protegido", Detail: "/p/b/x", Size: 1 << 30, Group: "projeto b",
			Locked: true, LockReason: "dado insubstituivel"},
	}
}

func send(m tea.Model, msgs ...tea.Msg) tea.Model {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func TestSelectorRendersAndSelects(t *testing.T) {
	s := NewSelector(NewPlainTheme(), "Artefatos", "3 itens", rows())
	send(s, tea.WindowSizeMsg{Width: 100, Height: 24})

	view := s.View().Content
	for _, want := range []string{"Artefatos", "node_modules", ".next", "projeto a", "3.0 GB"} {
		if !strings.Contains(view, want) {
			t.Errorf("a tela nao mostrou %q", want)
		}
	}

	// Mark the first row and confirm.
	send(s, key(" "), key("enter"))
	chosen := s.Chosen()
	if len(chosen) != 1 || chosen[0].ID != "a" {
		t.Fatalf("esperado o primeiro item marcado, obtido %+v", chosen)
	}
}

// A locked row is visible and unreachable. Select all must not pick it up.
func TestSelectorNeverReturnsLockedRows(t *testing.T) {
	s := NewSelector(NewPlainTheme(), "t", "", rows())
	send(s, tea.WindowSizeMsg{Width: 100, Height: 24}, key("a"), key("enter"))

	for _, r := range s.Chosen() {
		if r.Locked {
			t.Fatalf("item bloqueado foi selecionado: %s", r.ID)
		}
	}
	if len(s.Chosen()) != 2 {
		t.Fatalf("esperado 2 selecionaveis, obtido %d", len(s.Chosen()))
	}
}

// Quitting means quitting: nothing gets removed because the user walked away.
func TestSelectorCancelReturnsNothing(t *testing.T) {
	s := NewSelector(NewPlainTheme(), "t", "", rows())
	send(s, key("a"), key("q"))

	if !s.Cancelled() {
		t.Error("sair deveria contar como cancelamento")
	}
	if len(s.Chosen()) != 0 {
		t.Error("cancelar nao pode devolver selecao")
	}
}

// Confirming without marking anything returns nothing, rather than defaulting
// to everything.
func TestSelectorEmptyConfirmReturnsNothing(t *testing.T) {
	s := NewSelector(NewPlainTheme(), "t", "", rows())
	send(s, key("enter"))
	if len(s.Chosen()) != 0 {
		t.Error("confirmar sem marcar nada deveria devolver vazio")
	}
}

// The cursor skips group headers, so it always sits on something selectable.
func TestCursorSkipsHeaders(t *testing.T) {
	s := NewSelector(NewPlainTheme(), "t", "", rows())
	send(s, tea.WindowSizeMsg{Width: 100, Height: 24})

	for i := 0; i < 6; i++ {
		if s.lines[s.cursor].kind != lineRow {
			t.Fatalf("cursor parou num cabecalho apos %d movimentos", i)
		}
		send(s, key("j"))
	}
}
