package cli

import (
	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/modules"
)

func appsCmd() *cobra.Command {
	var deep, zap bool

	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Desinstala programas, do mais pesado ao mais leve",
		Long: "Lista os aplicativos instalados ordenados pelo espaco que ocupam de\n" +
			"verdade, somando o app e os arquivos que ele espalhou pelo sistema.\n" +
			"Marque os que quiser, confira o que sera removido e desinstale tudo\n" +
			"de uma vez.",
		RunE: func(cmd *cobra.Command, args []string) error {
			env := g.env()
			env.Deep = deep
			env.Zap = zap
			return g.runModule(cmd.Context(), modules.KindApps, env)
		},
	}

	cmd.Flags().BoolVar(&deep, "deep", false,
		"tambem procura pastas com o nome do app, nao so pelo identificador")
	cmd.Flags().BoolVar(&zap, "zap", false,
		"em casks do Homebrew, usa a lista completa de remocao do brew")
	return cmd
}
