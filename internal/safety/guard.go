// Package safety decides what macsweep is allowed to delete.
//
// Every destructive path in the program funnels through Guard.Check. The
// rule is deliberately inverted from what a cleaner usually does: nothing is
// deletable unless it sits under a known root, and a deny match overrides any
// allow, however specific the allow was. When in doubt the answer is no.
package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Level says how much ceremony a path needs before it can be removed.
type Level int

const (
	// Denied means never, under any flag or confirmation.
	Denied Level = iota
	// Sensitive means it holds real user data and may only be removed when
	// something matched it exactly, never by a heuristic.
	Sensitive
	// NeedsAdmin means the current user does not own it. macsweep never
	// escalates on its own, it prints the command for the user to run.
	NeedsAdmin
	// Allowed means it can be removed after the usual confirmation.
	Allowed
)

func (l Level) String() string {
	switch l {
	case Denied:
		return "negado"
	case Sensitive:
		return "sensivel"
	case NeedsAdmin:
		return "exige admin"
	default:
		return "liberado"
	}
}

// Match says how confidently the caller identified this path.
type Match int

const (
	// Heuristic is a guess: a name that looked similar, a substring hit.
	Heuristic Match = iota
	// Exact is a deterministic identification: a bundle id that names the
	// directory, a signing team id, a build artifact whose exact path the
	// project scanner resolved.
	Exact
)

// Verdict is the answer for one path.
type Verdict struct {
	Level  Level
	Reason string
}

func (v Verdict) OK() bool { return v.Level == Allowed }

// Permits reports whether a caller holding this kind of match may remove the
// path. A sensitive path opens up only to an exact identification, which is
// what keeps a fuzzy app-name match away from someone's Documents.
func (v Verdict) Permits(m Match) bool {
	switch v.Level {
	case Allowed:
		return true
	case Sensitive:
		return m == Exact
	default:
		return false
	}
}

// Guard holds the rules. Build one with New and share it.
type Guard struct {
	home   string
	uid    int
	ignore []string
}

// New builds a Guard for the current user. ignore is the user's own list of
// paths to leave alone, from the config file.
func New(home string, uid int, ignore []string) *Guard {
	g := &Guard{home: filepath.Clean(home), uid: uid}
	for _, p := range ignore {
		if p = strings.TrimSpace(p); p != "" {
			g.ignore = append(g.ignore, filepath.Clean(expand(p, home)))
		}
	}
	return g
}

func expand(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// roots are the only places macsweep will ever remove anything from.
func (g *Guard) roots() []string {
	return []string{
		g.home,
		"/Applications",
		"/Library/Application Support",
		"/Library/Caches",
		"/Library/Logs",
		"/Library/LaunchAgents",
		"/Library/LaunchDaemons",
		"/private/var/folders", // per user cache and temp, via DARWIN_USER_CACHE_DIR
		"/private/tmp",
	}
}

// denied lists what must survive a cleaner, no matter what asked for it.
// These are relative to the home directory unless they start with a slash.
var denied = []string{
	"Library/Keychains",
	"Library/Mobile Documents",                  // iCloud Drive
	"Library/Application Support/MobileSync",    // iOS backups, unrecoverable
	"Library/Application Support/com.apple.TCC", // privacy database
	"Library/Photos",
	"Pictures/Photos Library.photoslibrary",
	".ssh",
	".gnupg",
	".aws",
	".config/gh",
	".local/state/macsweep", // our own undo manifests
}

// sensitive holds real user data that may only be removed on an exact match.
var sensitive = []string{
	"Library/Group Containers",
	"Library/Containers",
	"Library/Messages",
	"Library/Mail",
	"Library/Safari",
	"Documents",
	"Desktop",
	"Downloads",
	"Movies",
	"Music",
	"Pictures",
}

// Check answers whether path may be removed.
func (g *Guard) Check(path string) Verdict {
	if !filepath.IsAbs(path) {
		return Verdict{Denied, "caminho relativo"}
	}
	clean := filepath.Clean(path)

	if clean == "/" || clean == g.home {
		return Verdict{Denied, "raiz do sistema ou da pasta pessoal"}
	}

	// A symlink is removed as a link, never followed, but a path that reaches
	// its target through a symlinked parent could sidestep every rule below.
	// Resolve it and judge both spellings.
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(clean)); err == nil {
		if real := filepath.Join(resolved, filepath.Base(clean)); real != clean {
			if v := g.classify(real); !v.OK() {
				return Verdict{v.Level, v.Reason + " (via symlink)"}
			}
		}
	}

	return g.classify(clean)
}

func (g *Guard) classify(clean string) Verdict {
	// The user's own ignore list outranks everything.
	for _, ig := range g.ignore {
		if under(clean, ig) {
			return Verdict{Denied, "na sua lista de ignore"}
		}
	}

	// Deny wins, so it is evaluated before any allow.
	for _, d := range denied {
		abs := d
		if !filepath.IsAbs(d) {
			abs = filepath.Join(g.home, d)
		}
		if under(clean, abs) {
			return Verdict{Denied, "dado insubstituivel: " + d}
		}
	}

	// A container's Documents folder is the app's user data, whatever the app.
	if strings.Contains(clean, "/Data/Documents") {
		return Verdict{Denied, "documentos do usuario dentro do container"}
	}

	inRoot := false
	for _, r := range g.roots() {
		if clean == r {
			return Verdict{Denied, "raiz protegida: " + r}
		}
		if under(clean, r) {
			inRoot = true
			break
		}
	}
	if !inRoot {
		return Verdict{Denied, "fora das pastas que o macsweep gerencia"}
	}

	for _, s := range sensitive {
		abs := s
		if !filepath.IsAbs(s) {
			abs = filepath.Join(g.home, s)
		}
		if clean == abs {
			return Verdict{Denied, "raiz protegida: " + s}
		}
		if under(clean, abs) {
			return Verdict{Sensitive, "guarda dados de usuario, exige correspondencia exata"}
		}
	}

	if fi, err := os.Lstat(clean); err == nil {
		if st := owner(fi); st != nil {
			if int(*st) != g.uid {
				return Verdict{NeedsAdmin, "pertence a outro usuario"}
			}
		}
		if restricted(fi) {
			return Verdict{Denied, "protegido pelo SIP"}
		}
	}

	return Verdict{Allowed, ""}
}

// under reports whether path is base or sits inside it, comparing whole path
// components so that /a/bc never counts as being under /a/b.
func under(path, base string) bool {
	if path == base {
		return true
	}
	if !strings.HasSuffix(base, string(filepath.Separator)) {
		base += string(filepath.Separator)
	}
	return strings.HasPrefix(path, base)
}

// Describe renders a verdict for the user.
func (v Verdict) Describe(path string) string {
	if v.OK() {
		return path
	}
	return fmt.Sprintf("%s [%s: %s]", path, v.Level, v.Reason)
}
