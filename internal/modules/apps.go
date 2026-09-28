package modules

import (
	"context"
	"fmt"

	"github.com/brunogallotte/macsweep/internal/apps"
	"github.com/brunogallotte/macsweep/internal/safety"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// ScanApps lists installed applications by what they really cost: the bundle
// plus everything they scattered around the system.
func ScanApps(ctx context.Context, env *Env) (Result, error) {
	env.say("lendo aplicativos instalados")

	list, err := apps.Discover(ctx, apps.Options{Deep: env.Deep})
	if err != nil {
		return Result{}, err
	}

	byID := make(map[string]*apps.App, len(list))
	rows := make([]ui.Row, 0, len(list))
	var total int64

	for _, a := range list {
		byID[a.BundleID] = a
		total += a.Footprint()

		note := string(a.Source)
		if n := a.LeftoverSize(); n > 0 {
			note = fmt.Sprintf("%s  +%s de residuos", note, ui.Size(n))
		}
		row := ui.Row{
			ID:     a.BundleID,
			Title:  a.Name,
			Detail: a.Path,
			Note:   note,
			Size:   a.Footprint(),
		}

		switch {
		case a.Running:
			row.Locked, row.LockReason = true, "em execucao, feche antes"
		case !a.Removable():
			row.Locked, row.LockReason = true, "gerenciado pelo Setapp"
		default:
			if v := env.Guard.Check(a.Path); !v.Permits(safety.Exact) {
				row.Locked, row.LockReason = true, v.Reason
			}
		}
		rows = append(rows, row)
	}

	return Result{
		Title:    "Aplicativos instalados",
		Subtitle: fmt.Sprintf("%d apps, %s no total", len(list), ui.Size(total)),
		Rows:     rows,
		Preview:  true,
		Expand: func(chosen []ui.Row) ([]ui.Row, []Command) {
			var items []ui.Row
			var cmds []Command
			for _, r := range chosen {
				a := byID[r.ID]
				if a == nil {
					continue
				}
				// A cask knows exactly what it installed and how to undo it,
				// including a curated leftover list. Deferring to brew beats
				// anything macsweep could infer.
				if a.Source == apps.SourceCask {
					argv := []string{"uninstall", "--cask"}
					if env.Zap {
						argv = append(argv, "--zap")
					}
					cmds = append(cmds, Command{
						Title: a.Name + " (Homebrew)",
						Argv:  append([]string{"brew"}, append(argv, a.CaskToken)...),
					})
					continue
				}
				items = append(items, expandApp(env, a)...)
			}
			return items, cmds
		},
	}, nil
}

// expandApp turns one app into the concrete paths to remove, dropping
// anything the guard refuses at the confidence the match carries.
func expandApp(env *Env, a *apps.App) []ui.Row {
	group := fmt.Sprintf("%s  ·  %s", a.Name, ui.Size(a.Footprint()))

	out := []ui.Row{{
		ID:       a.Path,
		Title:    "aplicativo",
		Detail:   a.Path,
		Note:     string(a.Source),
		Size:     a.BundleSize,
		Group:    group,
		Selected: true,
	}}

	for _, l := range a.Leftovers {
		match := safety.Exact
		if l.Confidence == apps.ByName {
			match = safety.Heuristic
		}
		if !env.Guard.Check(l.Path).Permits(match) {
			continue
		}
		out = append(out, ui.Row{
			ID:     l.Path,
			Title:  l.Kind,
			Detail: l.Path,
			Note:   string(l.Confidence),
			Size:   l.Size,
			Group:  group,
			// A name based guess is never removed unless the user ticks it.
			Selected: l.Confidence == apps.Exact,
		})
	}
	return out
}
