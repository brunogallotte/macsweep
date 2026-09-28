package modules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/brunogallotte/macsweep/internal/projects"
	"github.com/brunogallotte/macsweep/internal/safety"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// DefaultRoots are the places code tends to live. ~/Documents is included on
// purpose: it is where plenty of people keep repositories, and the other
// tools in this space do not look there.
func DefaultRoots(home string) []string {
	candidates := []string{
		"Documents", "Projects", "Code", "dev", "Developer",
		"GitHub", "Workspace", "src", "repos", "work",
	}
	var out []string
	for _, c := range candidates {
		p := filepath.Join(home, c)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// ScanProjects finds regenerable build output in the user's repositories.
func ScanProjects(ctx context.Context, env *Env) (Result, error) {
	roots := env.Roots
	if len(roots) == 0 {
		roots = DefaultRoots(env.Home)
	}
	if len(roots) == 0 {
		return Result{}, fmt.Errorf("nenhuma pasta de projetos encontrada")
	}

	idleDays := env.IdleDays
	if idleDays <= 0 {
		idleDays = 30
	}
	idle := time.Duration(idleDays) * 24 * time.Hour

	var all []*projects.Project
	var denied int64
	for _, root := range roots {
		env.say("escaneando " + shorten(root, env.Home))
		ps, stats, err := projects.Discover(ctx, root)
		if err != nil {
			continue
		}
		all = append(all, ps...)
		denied += stats.Denied
	}

	var rows []ui.Row
	for _, p := range all {
		group := fmt.Sprintf("%s  ·  %s  ·  ocioso ha %s",
			p.Name, ui.Size(p.Size()), Days(p.Idle()))

		for _, a := range p.Artifacts {
			row := ui.Row{
				ID:     a.Path,
				Title:  a.Kind,
				Detail: a.Path,
				Note:   a.Regenerate,
				Size:   a.Size,
				Group:  group,
				// Idleness of the whole project is the useful signal, not of
				// one directory inside it.
				Selected: p.Idle() > idle,
			}
			// The scanner resolved this exact path, so it is not a guess.
			if v := env.Guard.Check(a.Path); !v.Permits(safety.Exact) {
				row.Locked, row.LockReason, row.Selected = true, v.Reason, false
			}
			rows = append(rows, row)
		}
	}

	sub := fmt.Sprintf("%d projetos, %d artefatos", len(all), len(rows))
	if denied > 0 {
		sub += fmt.Sprintf(", %d caminhos sem permissao", denied)
	}
	return Result{Title: "Artefatos de projeto", Subtitle: sub, Rows: rows}, nil
}

// Days renders a duration the way a person talks about it.
func Days(d time.Duration) string {
	n := int(d.Hours() / 24)
	switch {
	case n <= 0:
		return "hoje"
	case n == 1:
		return "1 dia"
	case n < 30:
		return fmt.Sprintf("%d dias", n)
	case n < 365:
		return fmt.Sprintf("%d meses", n/30)
	default:
		return fmt.Sprintf("%.1f anos", float64(n)/365)
	}
}

func shorten(path, home string) string {
	if len(path) > len(home) && path[:len(home)] == home {
		return "~" + path[len(home):]
	}
	return path
}
