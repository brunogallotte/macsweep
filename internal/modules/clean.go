package modules

import (
	"context"
	"fmt"

	"github.com/brunogallotte/macsweep/internal/rules"
	"github.com/brunogallotte/macsweep/internal/safety"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// ScanClean measures every cleaning rule at or below the configured risk.
func ScanClean(ctx context.Context, env *Env) (Result, error) {
	risk := env.Risk
	if risk == "" {
		risk = rules.Moderate
	}

	env.say("medindo caches conhecidos")
	found := rules.Scan(ctx, env.Home, risk)

	commands := map[string]Command{}
	var rows []ui.Row

	for _, f := range found {
		group := fmt.Sprintf("%s  ·  %s", f.Rule.Tool, f.Rule.Risk)

		// A rule with a command is handed back to the tool that owns the
		// data instead of having its files deleted underneath it.
		if len(f.Rule.Command) > 0 {
			id := "cmd:" + f.Rule.ID
			commands[id] = Command{Title: f.Rule.Title, Argv: f.Rule.Command}
			rows = append(rows, ui.Row{
				ID:       id,
				Title:    f.Rule.Title,
				Detail:   argvLine(f.Rule.Command),
				Note:     f.Rule.Regenerate,
				Size:     f.Size,
				Group:    group,
				Selected: f.Rule.Risk == rules.Safe,
			})
			continue
		}

		row := ui.Row{
			ID:       f.Path,
			Title:    f.Rule.Title,
			Detail:   f.Path,
			Note:     f.Rule.Regenerate,
			Size:     f.Size,
			Group:    group,
			Selected: f.Rule.Risk == rules.Safe,
		}
		// The catalogue names this path outright, so it counts as exact.
		if v := env.Guard.Check(f.Path); !v.Permits(safety.Exact) {
			row.Locked, row.LockReason, row.Selected = true, v.Reason, false
		}
		rows = append(rows, row)
	}

	return Result{
		Title:    "Caches",
		Subtitle: fmt.Sprintf("%d itens medidos", len(rows)),
		Rows:     rows,
		Expand: func(chosen []ui.Row) ([]ui.Row, []Command) {
			var items []ui.Row
			var cmds []Command
			for _, r := range chosen {
				if c, ok := commands[r.ID]; ok {
					cmds = append(cmds, c)
					continue
				}
				items = append(items, r)
			}
			return items, cmds
		},
	}, nil
}

func argvLine(argv []string) string {
	out := ""
	for i, a := range argv {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
