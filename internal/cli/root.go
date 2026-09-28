// Package cli wires the modules to the terminal.
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/app"
	"github.com/brunogallotte/macsweep/internal/safety"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// Version is stamped at build time.
var Version = "dev"

type globals struct {
	dryRun    bool
	jsonOut   bool
	assumeYes bool
	permanent bool
	stateDir  string

	theme *ui.Theme
	guard *safety.Guard
	home  string
}

var g = &globals{}

// Execute runs the CLI.
func Execute() error {
	root := &cobra.Command{
		Use:   "macsweep",
		Short: "Recupera espaco no seu Mac sem adivinhacao",
		Long: "macsweep encontra o que ocupa espaco no seu Mac, mostra de onde veio\n" +
			"e remove com seguranca: tudo vai para a Lixeira e toda operacao pode\n" +
			"ser desfeita com `macsweep undo`.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return g.setup()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// With a terminal to draw on, macsweep opens as an app you stay
			// inside. Piped or asked for JSON, it prints the same summary and
			// exits, so it stays usable from a script.
			if !g.interactive() {
				return runPanel(cmd)
			}
			return app.Run(cmd.Context(), app.Options{
				Env:       g.env(),
				Theme:     g.theme,
				Home:      g.home,
				StateDir:  g.stateDir,
				DryRun:    g.dryRun,
				Permanent: g.permanent,
			})
		},
	}

	pf := root.PersistentFlags()
	pf.BoolVar(&g.dryRun, "dry-run", false, "mostra o que seria removido sem tocar em nada")
	pf.BoolVar(&g.jsonOut, "json", false, "saida em JSON, para script")
	pf.BoolVarP(&g.assumeYes, "yes", "y", false, "nao pergunta, usa a pre-selecao")
	pf.BoolVar(&g.permanent, "delete", false, "apaga de vez em vez de mandar para a Lixeira")
	pf.StringVar(&g.stateDir, "state-dir", "", "onde gravar os manifestos de undo")

	root.AddCommand(projectsCmd(), analyzeCmd(), appsCmd(), cleanCmd(), undoCmd(), emptyCmd(), doctorCmd())
	return root.Execute()
}

func (g *globals) setup() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	g.home = home

	uid := os.Getuid()
	if u, err := user.Current(); err == nil {
		if n, err := strconv.Atoi(u.Uid); err == nil {
			uid = n
		}
	}

	g.guard = safety.New(home, uid, loadIgnore(home))
	if ui.Plain() {
		g.theme = ui.NewPlainTheme()
	} else {
		g.theme = ui.NewTheme(true)
	}
	return nil
}

// progress writes a transient status line to stderr, and only when stderr is
// a terminal. Redirected into a file or a pipe, the carriage returns and
// erase sequences would be written out literally.
func (g *globals) progress(format string, args ...any) {
	if !stderrIsTTY() {
		return
	}
	fmt.Fprintf(os.Stderr, "\r"+format, args...)
}

func (g *globals) clearProgress() {
	if stderrIsTTY() {
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
}

var stderrIsTTY = sync.OnceValue(func() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
})

// interactive reports whether a selection UI can be shown at all.
func (g *globals) interactive() bool {
	return !g.jsonOut && !g.assumeYes && !ui.Plain()
}

// choose runs the selector, or falls back to the preselection when there is
// no terminal to draw on. A script gets exactly what an interactive user
// would have seen marked by default.
func (g *globals) choose(title, subtitle string, rows []ui.Row) ([]ui.Row, error) {
	if !g.interactive() {
		var out []ui.Row
		for _, r := range rows {
			if r.Selected && !r.Locked {
				out = append(out, r)
			}
		}
		return out, nil
	}

	return ui.RunSelector(g.theme, title, subtitle, rows)
}

// snapshots lists APFS local snapshots. Their presence is the single most
// common reason a user deletes 40 GB and sees the free space barely move.
func snapshots() []string {
	out, err := exec.Command("tmutil", "listlocalsnapshots", "/").Output()
	if err != nil {
		return nil
	}
	var snaps []string
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "com.apple.") {
			snaps = append(snaps, l)
		}
	}
	return snaps
}

func loadIgnore(home string) []string {
	data, err := os.ReadFile(configPath(home))
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			out = append(out, l)
		}
	}
	return out
}

func configPath(home string) string {
	return home + "/.config/macsweep/ignore"
}
