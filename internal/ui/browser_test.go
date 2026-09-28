package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/brunogallotte/macsweep/internal/scan"
)

func tree() *scan.Node {
	root := &scan.Node{Name: "raiz", Path: "/raiz", IsDir: true, Size: 10 << 30}
	big := &scan.Node{Name: "grande", Path: "/raiz/grande", IsDir: true, Size: 8 << 30, Parent: root}
	inner := &scan.Node{Name: "dentro", Path: "/raiz/grande/dentro", IsDir: true, Size: 7 << 30, Parent: big}
	big.Children = []*scan.Node{inner}
	small := &scan.Node{Name: "pequeno", Path: "/raiz/pequeno", IsDir: true, Size: 2 << 30, Parent: root}
	root.Children = []*scan.Node{big, small}
	return root
}

func TestBrowserNavigatesAndMarks(t *testing.T) {
	b := NewBrowser(NewPlainTheme(), tree())
	send(b, tea.WindowSizeMsg{Width: 100, Height: 24})

	view := b.View().Content
	if !strings.Contains(view, "grande/") || !strings.Contains(view, "8.0 GB") {
		t.Fatalf("a tela inicial nao lista os filhos:\n%s", view)
	}

	// Descend, confirm the path changed, then come back up.
	send(b, key("l"))
	if !strings.Contains(b.View().Content, "/raiz/grande") {
		t.Error("nao entrou no diretorio")
	}
	send(b, key("h"))
	if !strings.Contains(b.View().Content, "grande/") {
		t.Error("nao voltou para o diretorio anterior")
	}

	// Mark the biggest child and read it back.
	send(b, key(" "))
	chosen := b.Chosen()
	if len(chosen) != 1 || chosen[0].Detail != "/raiz/grande" {
		t.Fatalf("marcacao nao voltou como esperado: %+v", chosen)
	}
}

func TestBrowserQuitDiscardsMarks(t *testing.T) {
	b := NewBrowser(NewPlainTheme(), tree())
	send(b, key(" "), key("q"))
	if len(b.Chosen()) != 0 {
		t.Error("sair deveria descartar as marcacoes")
	}
}

// Going up from the root must not panic or escape the tree.
func TestBrowserStopsAtRoot(t *testing.T) {
	b := NewBrowser(NewPlainTheme(), tree())
	send(b, key("h"), key("h"), key("h"))
	if !strings.Contains(b.View().Content, "/raiz") {
		t.Error("subir demais saiu da arvore")
	}
}
