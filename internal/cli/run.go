package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/brunogallotte/macsweep/internal/modules"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// env builds the module environment from the global flags.
func (g *globals) env() *modules.Env {
	return &modules.Env{
		Home:     g.home,
		Guard:    g.guard,
		Progress: func(s string) { g.progress("%s", g.theme.Subtle.Render(s+"...")) },
	}
}

// runModule is the one shot version of what the interactive app does: scan,
// let the user choose, expand, confirm, apply, report. Both paths go through
// internal/modules, so a subcommand and the app can never disagree.
func (g *globals) runModule(ctx context.Context, kind modules.Kind, env *modules.Env) error {
	res, err := modules.Scan(ctx, kind, env)
	g.clearProgress()
	if err != nil {
		return err
	}

	// Without --yes, asking for JSON means "show me what you found", not
	// "go ahead and delete it".
	if g.jsonOut && !g.assumeYes {
		return json.NewEncoder(os.Stdout).Encode(res.Rows)
	}
	if res.Empty() {
		fmt.Println(g.theme.Subtle.Render("nada encontrado"))
		return nil
	}

	chosen, err := g.choose(res.Title, res.Subtitle, res.Rows)
	if err != nil {
		return err
	}
	if len(chosen) == 0 {
		fmt.Println(g.theme.Subtle.Render("nada selecionado"))
		return nil
	}

	items, cmds := chosen, []modules.Command(nil)
	if res.Expand != nil {
		items, cmds = res.Expand(chosen)
	}

	// The uninstaller shows every path before anything moves.
	if res.Preview && len(items) > 0 && g.interactive() {
		items, err = g.choose("Confira antes de remover", sizeLine(items),
			ui.PreviewRows(items, g.home))
		if err != nil {
			return err
		}
	}
	if len(items) == 0 && len(cmds) == 0 {
		fmt.Println(g.theme.Subtle.Render("nada selecionado"))
		return nil
	}

	return g.report(modules.Apply(items, cmds, g.applyOptions(string(kind), kind == modules.KindEmpty)))
}

func (g *globals) applyOptions(command string, purge bool) modules.ApplyOptions {
	return modules.ApplyOptions{
		Command:   command,
		Home:      g.home,
		StateDir:  g.stateDir,
		DryRun:    g.dryRun,
		Permanent: g.permanent,
		Purge:     purge,
	}
}

func (g *globals) report(out ui.Outcome) error {
	if g.jsonOut {
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	fmt.Print(g.theme.Render(out))
	return nil
}

func sizeLine(rows []ui.Row) string {
	var n int64
	for _, r := range rows {
		n += r.Size
	}
	return fmt.Sprintf("%s em %d itens", ui.Size(n), len(rows))
}
