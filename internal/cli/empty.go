package cli

import (
	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/modules"
)

func emptyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "empty",
		Short: "Esvazia da Lixeira o que o macsweep mandou para la",
		Long: "Mover para a Lixeira nao libera espaco: e um rename dentro do mesmo\n" +
			"volume. Este comando apaga de vez o que o macsweep mandou para la e\n" +
			"mostra o ganho real, medido no disco.\n\n" +
			"Mexe apenas nos itens registrados nos manifestos, entao o que voce\n" +
			"jogou fora pela sua conta continua na Lixeira, intacto.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return g.runModule(cmd.Context(), modules.KindEmpty, g.env())
		},
	}
}
