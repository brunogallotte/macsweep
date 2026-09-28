package cli

import (
	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/modules"
)

func projectsCmd() *cobra.Command {
	var idleDays int

	cmd := &cobra.Command{
		Use:   "projects [raiz...]",
		Short: "Artefatos de build regeneraveis nos seus repositorios",
		Long: "Varre seus repositorios e lista node_modules, .next, target, obj e\n" +
			"companhia, agrupados por projeto, com o comando que regenera cada um.\n" +
			"Vem marcado por padrao o que nao e tocado ha mais de 30 dias.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := g.env()
			env.Roots = args
			env.IdleDays = idleDays
			return g.runModule(cmd.Context(), modules.KindProjects, env)
		},
	}

	cmd.Flags().IntVar(&idleDays, "idle-days", 30,
		"marca automaticamente artefatos sem uso ha mais dias que isto")
	return cmd
}
