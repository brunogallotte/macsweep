package modules

import (
	"context"
	"fmt"

	"github.com/brunogallotte/macsweep/internal/trash"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// ScanEmpty lists what macsweep put in the Trash and has not yet purged.
//
// This is the module that actually gives space back. Everything else only
// moves bytes to a different folder on the same volume.
func ScanEmpty(ctx context.Context, env *Env) (Result, error) {
	pending, total, err := trash.PendingItems("")
	if err != nil {
		return Result{}, err
	}

	rows := make([]ui.Row, 0, len(pending))
	for _, p := range pending {
		rows = append(rows, ui.Row{
			ID:       p.Trashed,
			Title:    p.Label,
			Detail:   p.Trashed,
			Note:     "veio de " + shorten(p.Path, env.Home),
			Size:     p.Size,
			Group:    "operacao " + p.BatchID,
			Selected: true,
		})
	}

	return Result{
		Title:    "Esvaziar a Lixeira",
		Subtitle: fmt.Sprintf("%s em %d itens, isto nao tem volta", ui.Size(total), len(rows)),
		Rows:     rows,
	}, nil
}
