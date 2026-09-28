package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brunogallotte/macsweep/internal/scan"
	"github.com/brunogallotte/macsweep/internal/ui"
)

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Checa o ambiente e explica o que pode atrapalhar a limpeza",
		RunE: func(cmd *cobra.Command, args []string) error {
			t := g.theme
			fmt.Println(t.Title.Render("macsweep doctor"))
			fmt.Println()

			if v, err := scan.VolumeAt(g.home); err == nil {
				used := v.Total - v.Free
				fmt.Printf("  %-22s %s livres de %s\n",
					"disco",
					t.Size.Render(ui.Size(v.Free)), ui.Size(v.Total))
				fmt.Printf("  %-22s %s\n", "", t.SizeBar(used, v.Total, 40))
			}

			// Full Disk Access is granted to the terminal, not to macsweep,
			// and it cannot be requested programmatically. So the check is a
			// real read of paths the tool needs, and the guidance names the
			// app the user has to tick.
			denied := probeDenied()
			fmt.Println()
			if len(denied) == 0 {
				fmt.Printf("  %s %s\n", t.Good.Render("✓"), "acesso total ao disco: concedido")
			} else {
				fmt.Printf("  %s %s\n", t.Warn.Render("!"), "acesso total ao disco: faltando")
				fmt.Printf("    %s\n", t.Subtle.Render(
					fmt.Sprintf("%d pastas nao puderam ser lidas, entao os totais saem por baixo", len(denied))))
				for _, p := range denied {
					fmt.Printf("      %s\n", t.Subtle.Render(strings.Replace(p, g.home, "~", 1)))
				}
				fmt.Println()
				fmt.Printf("    conceda para %s em\n", t.Accent.Render(terminalApp()))
				fmt.Printf("    %s\n", t.Subtle.Render("Ajustes > Privacidade e Seguranca > Acesso total ao disco"))
				fmt.Printf("    %s\n", t.Subtle.Render("depois feche e reabra o terminal por completo, nao basta uma aba nova"))
			}

			// Snapshots are the reason a cleanup can free gigabytes and move
			// the free-space number by nothing at all.
			snaps := snapshots()
			fmt.Println()
			if len(snaps) == 0 {
				fmt.Printf("  %s %s\n", t.Good.Render("✓"), "sem snapshots locais do APFS")
			} else {
				fmt.Printf("  %s %d snapshots locais do APFS\n", t.Warn.Render("!"), len(snaps))
				fmt.Printf("    %s\n", t.Subtle.Render(
					"eles seguram os blocos do que voce apagar, as vezes por ate 24h."))
				fmt.Printf("    %s\n", t.Subtle.Render(
					"para liberar antes disso: sudo tmutil thinlocalsnapshots / 10000000000 4"))
			}

			fmt.Println()
			fmt.Printf("  %-22s %s\n", "versao", Version)
			return nil
		},
	}
}

// probePaths are things macsweep genuinely needs to size. Probing what the
// tool uses, rather than the TCC database, keeps this correct across macOS
// releases: Apple has already moved that database once and broken every tool
// that sniffed for it.
func probePaths() []string {
	return []string{
		filepath.Join(g.home, "Library/Containers/com.apple.Safari/Data"),
		filepath.Join(g.home, "Library/Cookies"),
		filepath.Join(g.home, "Library/Application Support/MobileSync"),
		filepath.Join(g.home, ".Trash"),
	}
}

// probeDenied returns the paths that exist but cannot be read. A stat is not
// enough: several of these stat fine and only fail on open.
func probeDenied() []string {
	var out []string
	for _, p := range probePaths() {
		if _, err := os.Stat(p); err != nil {
			continue // genuinely absent, not blocked
		}
		f, err := os.Open(p)
		if err != nil {
			out = append(out, p)
			continue
		}
		if _, err := f.ReadDir(1); err != nil && !strings.Contains(err.Error(), "EOF") {
			out = append(out, p)
		}
		f.Close()
	}
	return out
}

// terminalApp names the app that actually needs the permission. Full Disk
// Access is granted to the terminal emulator, never to macsweep itself, so
// naming the wrong app sends the user looking in the wrong list.
//
// TERM_PROGRAM is set by every mainstream terminal and is far more reliable
// than walking the process tree, which breaks the moment a shell, a
// multiplexer or an editor sits in between.
func terminalApp() string {
	switch os.Getenv("TERM_PROGRAM") {
	case "Apple_Terminal":
		return "Terminal"
	case "iTerm.app":
		return "iTerm"
	case "WarpTerminal", "Warp":
		return "Warp"
	case "ghostty":
		return "Ghostty"
	case "vscode":
		return "Visual Studio Code"
	case "Hyper":
		return "Hyper"
	case "WezTerm":
		return "WezTerm"
	case "kitty":
		return "kitty"
	case "Alacritty":
		return "Alacritty"
	}
	return terminalAppFromProcessTree()
}

func terminalAppFromProcessTree() string {
	pid := os.Getppid()
	for i := 0; i < 6 && pid > 1; i++ {
		out, err := exec.Command("ps", "-o", "comm=,ppid=", "-p", fmt.Sprint(pid)).Output()
		if err != nil {
			break
		}
		fields := strings.Fields(string(out))
		if len(fields) == 0 {
			break
		}
		path := strings.Join(fields[:len(fields)-1], " ")
		if i := strings.Index(path, ".app/"); i >= 0 {
			return filepath.Base(path[:i+4])
		}
		if next := fields[len(fields)-1]; next != "" {
			fmt.Sscan(next, &pid)
		} else {
			break
		}
	}
	return "seu terminal"
}
