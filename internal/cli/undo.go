package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/trash"
	"github.com/brunogallotte/macsweep/internal/ui"
)

func undoCmd() *cobra.Command {
	var list bool

	cmd := &cobra.Command{
		Use:   "undo [id]",
		Short: "Devolve ao lugar o que uma operacao removeu",
		Long: "Sem argumento, desfaz a ultima operacao. Cada execucao do macsweep\n" +
			"grava um manifesto com o caminho original de tudo que removeu, entao\n" +
			"o undo funciona mesmo quando o \"Colocar de volta\" do Finder nao ajuda.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t := g.theme

			if list {
				batches, err := trash.History(g.stateDir)
				if err != nil {
					return err
				}
				if g.jsonOut {
					return json.NewEncoder(os.Stdout).Encode(batches)
				}
				if len(batches) == 0 {
					fmt.Println(t.Subtle.Render("nenhuma operacao registrada"))
					return nil
				}
				for _, b := range batches {
					status := ""
					if b.Permanent {
						status = t.Warn.Render("  (permanente, nao reversivel)")
					}
					fmt.Printf("%s  %-10s  %3d itens  %10s%s\n",
						t.Accent.Render(b.ID), b.Command, len(b.Outcomes),
						ui.Size(b.Reclaimed()), status)
				}
				return nil
			}

			id := ""
			if len(args) == 1 {
				id = args[0]
			}
			n, err := trash.Undo(id, g.stateDir)
			if n > 0 {
				fmt.Printf("%s %d itens de volta no lugar\n", t.Good.Render("pronto:"), n)
			}
			if err != nil {
				return err
			}
			if n == 0 {
				fmt.Println(t.Subtle.Render("nada para restaurar"))
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&list, "list", "l", false, "lista as operacoes registradas")
	return cmd
}
