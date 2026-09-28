package apps

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/brunogallotte/macsweep/internal/scan"
)

// Confidence says how the file was tied to the app. It drives both what comes
// preselected and what the safety guard will allow.
type Confidence string

const (
	// Exact means the path is named after the bundle identifier, or a
	// receipt or cask definition listed it. There is no guesswork.
	Exact Confidence = "exato"
	// ByName means a folder simply carries the app's name. Useful, and the
	// source of every horror story in this category, so it is off by default.
	ByName Confidence = "por nome"
)

// Leftover is one file or folder an app left behind.
type Leftover struct {
	Path       string     `json:"path"`
	Kind       string     `json:"kind"`
	Size       int64      `json:"size"`
	Confidence Confidence `json:"confidence"`
}

// Options tunes how aggressively leftovers are matched.
type Options struct {
	// Deep turns on name based matching. It finds more and it is the tier
	// where cleaners historically deleted the wrong vendor's data, so the
	// results come in unselected and clearly marked.
	Deep bool
}

type leftoverIndex struct {
	home      string
	opts      Options
	cacheDir  string
	stopWords map[string]bool
	agents    []agent
}

type agent struct {
	path  string
	label string
	prog  string
}

// newLeftoverIndex prepares everything that is shared across apps: the launch
// agent table, the per user cache directory, and the stop word list.
//
// The stop words are computed from the machine rather than hard coded. Any
// bundle id component shared by several installed apps (com, apple, google,
// microsoft) is by definition useless for telling apps apart, and matching on
// one is how an uninstall reaches into a neighbour's data.
func newLeftoverIndex(opts Options, bundleIDs []string) *leftoverIndex {
	home, _ := os.UserHomeDir()

	counts := map[string]int{}
	for _, id := range bundleIDs {
		seen := map[string]bool{}
		for _, part := range strings.Split(strings.ToLower(id), ".") {
			if !seen[part] {
				seen[part] = true
				counts[part]++
			}
		}
	}
	stop := map[string]bool{}
	for part, n := range counts {
		if n > 2 || len(part) < 5 {
			stop[part] = true
		}
	}

	idx := &leftoverIndex{home: home, opts: opts, stopWords: stop}
	if out, err := exec.Command("getconf", "DARWIN_USER_CACHE_DIR").Output(); err == nil {
		idx.cacheDir = strings.TrimSpace(string(out))
	}
	idx.agents = loadAgents(home)
	return idx
}

// loadAgents reads the launch agent and daemon plists once. Matching on the
// Label and the executable inside, rather than on the file name, is what
// makes this reliable: plenty of agents are named nothing like their app.
func loadAgents(home string) []agent {
	var out []agent
	dirs := []string{
		filepath.Join(home, "Library", "LaunchAgents"),
		"/Library/LaunchAgents",
		"/Library/LaunchDaemons",
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := os.Open(path)
			if err != nil {
				continue
			}
			var job struct {
				Label   string   `plist:"Label"`
				Program string   `plist:"Program"`
				Args    []string `plist:"ProgramArguments"`
			}
			_ = plist.NewDecoder(f).Decode(&job)
			f.Close()

			prog := job.Program
			if prog == "" && len(job.Args) > 0 {
				prog = job.Args[0]
			}
			out = append(out, agent{path: path, label: job.Label, prog: prog})
		}
	}
	return out
}

// find returns everything tied to the app.
func (x *leftoverIndex) find(ctx context.Context, a *App) []Leftover {
	var out []Leftover
	seen := map[string]bool{}

	add := func(path, kind string, conf Confidence) {
		if path == "" || seen[path] {
			return
		}
		fi, err := os.Lstat(path)
		if err != nil {
			return
		}
		seen[path] = true

		size := scan.Allocated(fi)
		if fi.IsDir() {
			if res, err := scan.Walk(ctx, path, scan.Options{}); err == nil {
				size = res.Root.Size
			}
		}
		out = append(out, Leftover{Path: path, Kind: kind, Size: size, Confidence: conf})
	}

	id := a.BundleID
	lib := filepath.Join(x.home, "Library")

	// Tier 0: paths named after the bundle identifier. Deterministic.
	add(filepath.Join(lib, "Containers", id), "container", Exact)
	add(filepath.Join(lib, "Caches", id), "cache", Exact)
	add(filepath.Join(lib, "HTTPStorages", id), "cache web", Exact)
	add(filepath.Join(lib, "HTTPStorages", id+".binarycookies"), "cookies", Exact)
	add(filepath.Join(lib, "WebKit", id), "webkit", Exact)
	add(filepath.Join(lib, "Logs", id), "logs", Exact)
	add(filepath.Join(lib, "Preferences", id+".plist"), "preferencias", Exact)
	add(filepath.Join(lib, "Saved Application State", id+".savedState"), "estado da janela", Exact)
	add(filepath.Join(lib, "Application Scripts", id), "scripts", Exact)
	add(filepath.Join(lib, "Cookies", id+".binarycookies"), "cookies", Exact)
	if x.cacheDir != "" {
		add(filepath.Join(x.cacheDir, id), "cache temporario", Exact)
	}

	// Per host preferences carry a hardware uuid in the middle of the name.
	if entries, err := os.ReadDir(filepath.Join(lib, "Preferences", "ByHost")); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), id+".") {
				add(filepath.Join(lib, "Preferences", "ByHost", e.Name()), "preferencias", Exact)
			}
		}
	}

	// Group containers are prefixed with the developer's team id. Stripping
	// exactly one leading team component keeps this deterministic.
	if entries, err := os.ReadDir(filepath.Join(lib, "Group Containers")); err == nil {
		for _, e := range entries {
			if suffix, ok := stripTeamPrefix(e.Name()); ok {
				if suffix == id || suffix == "group."+id {
					add(filepath.Join(lib, "Group Containers", e.Name()), "group container", Exact)
				}
			}
		}
	}

	// Launch agents, matched on their contents.
	for _, ag := range x.agents {
		if ag.label == id || strings.HasPrefix(ag.label, id+".") ||
			(ag.prog != "" && strings.HasPrefix(ag.prog, a.Path+"/")) {
			add(ag.path, "inicializacao", Exact)
		}
	}

	// Files an installer package dropped, straight from its receipt.
	for _, pkgID := range a.PkgIDs {
		for _, p := range pkgFiles(pkgID) {
			add(p, "arquivo do instalador", Exact)
		}
	}

	if !x.opts.Deep {
		return out
	}

	// Tier 1: folders that simply carry the app's name. Equality only, never
	// a substring, and never a name that is too short or too common to
	// identify anything.
	if x.usableName(a.Name) {
		add(filepath.Join(lib, "Application Support", a.Name), "dados do app", ByName)
		add(filepath.Join(lib, "Logs", a.Name), "logs", ByName)
		add(filepath.Join(lib, "Caches", a.Name), "cache", ByName)
	}
	return out
}

// usableName rejects names too short or too generic to identify one app.
func (x *leftoverIndex) usableName(name string) bool {
	n := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	if len(n) < 5 {
		return false
	}
	return !x.stopWords[n]
}

// stripTeamPrefix removes a leading Apple team identifier, which is exactly
// ten uppercase alphanumerics followed by a dot.
func stripTeamPrefix(name string) (string, bool) {
	i := strings.Index(name, ".")
	if i != 10 {
		return "", false
	}
	for _, r := range name[:10] {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return "", false
		}
	}
	return name[11:], true
}

func pkgFiles(pkgID string) []string {
	out, err := exec.Command("pkgutil", "--files", pkgID, "--only-files").Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			paths = append(paths, filepath.Join("/", l))
		}
	}
	return paths
}
