package cli

import (
	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/modules"
	"github.com/brunogallotte/macsweep/internal/rules"
)

func cleanCmd() *cobra.Command {
	var risky, safeOnly bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Limpa caches de ferramentas, aplicativos e sistema",
		Long: "Varre o catalogo de caches conhecidos, mede cada um e mostra como\n" +
			"ele volta. O que e puro cache vem marcado; o que custa tempo para\n" +
			"reconstruir aparece desmarcado; o que pode fazer falta so aparece\n" +
			"com --risky.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := g.env()
			switch {
			case risky:
				env.Risk = rules.Risky
			case safeOnly:
				env.Risk = rules.Safe
			default:
				env.Risk = rules.Moderate
			}
			return g.runModule(cmd.Context(), modules.KindClean, env)
		},
	}

	cmd.Flags().BoolVar(&risky, "risky", false, "tambem mostra o que pode fazer falta")
	cmd.Flags().BoolVar(&safeOnly, "safe-only", false, "so o que e puro cache")
	return cmd
}
