package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/modules"
	"github.com/brunogallotte/macsweep/internal/scan"
	"github.com/brunogallotte/macsweep/internal/ui"
)

func analyzeCmd() *cobra.Command {
	var top int

	cmd := &cobra.Command{
		Use:   "analyze [caminho]",
		Short: "Mapa navegavel de onde foram os GB",
		Long: "Varre um caminho e abre um mapa navegavel, com o tamanho real em\n" +
			"disco em todos os niveis. Respeita arquivos esparsos e clones do\n" +
			"APFS, entao o numero bate com o que o disco realmente perdeu.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := g.theme
			root := g.home
			if len(args) == 1 {
				abs, err := filepath.Abs(args[0])
				if err != nil {
					return err
				}
				root = abs
			}

			start := time.Now()
			g.progress("%s", g.theme.Subtle.Render("escaneando "+root+"..."))
			res, err := scan.Walk(cmd.Context(), root, scan.Options{})
			if err != nil {
				return err
			}
			res.Root.SortBySize()
			g.clearProgress()

			if g.jsonOut {
				return json.NewEncoder(os.Stdout).Encode(summary(res, top))
			}

			if !g.interactive() {
				printSummary(res, top, time.Since(start))
				return nil
			}

			marked, err := ui.RunBrowser(t, res.Root)
			if err != nil {
				return err
			}
			if len(marked) == 0 {
				return nil
			}
			return g.report(modules.Apply(marked, nil, g.applyOptions("analyze", false)))
		},
	}

	cmd.Flags().IntVar(&top, "top", 20, "quantos maiores itens listar fora do modo interativo")
	return cmd
}

type summaryOut struct {
	Path    string     `json:"path"`
	Size    int64      `json:"size"`
	Files   int64      `json:"files"`
	Denied  int64      `json:"denied"`
	Biggest []nodeInfo `json:"biggest"`
}

type nodeInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func summary(res *scan.Result, top int) summaryOut {
	out := summaryOut{
		Path:   res.Root.Path,
		Size:   res.Root.Size,
		Files:  res.Stats.Files,
		Denied: res.Stats.Denied,
	}
	for _, n := range res.Root.Offenders(top) {
		out.Biggest = append(out.Biggest, nodeInfo{Path: n.Path, Size: n.Size})
	}
	return out
}

func printSummary(res *scan.Result, top int, took time.Duration) {
	t := g.theme
	fmt.Printf("%s  %s em %d arquivos  %s\n\n",
		t.Title.Render(res.Root.Path),
		t.Accent.Render(ui.Size(res.Root.Size)),
		res.Stats.Files,
		t.Subtle.Render(took.Round(time.Millisecond).String()))

	for _, n := range res.Root.Offenders(top) {
		fmt.Printf("  %10s  %s  %s\n",
			t.Size.Render(ui.Size(n.Size)),
			t.SizeBar(n.Size, res.Root.Size, 16),
			ui.TruncatePath(n.Path, 70))
	}

	if res.Stats.Denied > 0 {
		fmt.Printf("\n%s %s\n", t.Warn.Render("atencao:"), t.Subtle.Render(fmt.Sprintf(
			"%d caminhos sem permissao, o total esta por baixo. veja `macsweep doctor`",
			res.Stats.Denied)))
	}
}
