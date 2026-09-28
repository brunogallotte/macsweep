package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/scan"
	"github.com/brunogallotte/macsweep/internal/trash"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// runPanel is what `macsweep` with no arguments shows: where the disk stands
// and which door to open next.
func runPanel(cmd *cobra.Command) error {
	t := g.theme
	v, err := scan.VolumeAt(g.home)
	if err != nil {
		return err
	}
	used := v.Total - v.Free

	line := strings.Repeat("─", 58)
	fmt.Println()
	fmt.Printf("  %s\n", t.Title.Render("macsweep"))
	fmt.Printf("  %s\n", t.Subtle.Render(line))
	fmt.Printf("  %s livres de %s\n",
		t.Size.Render(ui.Size(v.Free)), ui.Size(v.Total))
	fmt.Printf("  %s\n\n", t.SizeBar(used, v.Total, 58))

	type entry struct{ cmd, desc string }
	entries := []entry{
		{"macsweep apps", "desinstala programas, do mais pesado ao mais leve"},
		{"macsweep clean", "limpa caches de ferramentas, apps e sistema"},
		{"macsweep projects", "artefatos de build regeneraveis nos repositorios"},
		{"macsweep analyze", "mapa navegavel de onde foram os GB"},
	}
	// Emptying only makes sense when there is something waiting, and it is
	// the only command that really gives space back.
	if _, inTrash, err := trash.PendingItems(g.stateDir); err == nil && inTrash > 0 {
		entries = append(entries, entry{
			"macsweep empty", "libera " + ui.Size(inTrash) + " que estao na Lixeira"})
	}
	for _, e := range entries {
		fmt.Printf("  %s  %s\n", t.Accent.Render(fmt.Sprintf("%-20s", e.cmd)), t.Subtle.Render(e.desc))
	}

	fmt.Println()
	if snaps := snapshots(); len(snaps) > 0 {
		fmt.Printf("  %s %s\n", t.Warn.Render("!"),
			t.Subtle.Render(fmt.Sprintf("%d snapshots locais podem segurar o espaco liberado, veja `macsweep doctor`", len(snaps))))
	}
	fmt.Printf("  %s\n", t.Subtle.Render("tudo vai para a Lixeira e pode ser revertido com `macsweep undo`"))
	fmt.Printf("  %s\n\n", t.Subtle.Render("num terminal, `macsweep` sozinho abre o painel interativo"))
	return nil
}
