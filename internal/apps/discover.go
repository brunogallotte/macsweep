// Package apps enumerates installed applications, measures what each one
// really costs on disk, and finds the files it left scattered around.
package apps

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"howett.net/plist"

	"github.com/brunogallotte/macsweep/internal/scan"
)

// Source says how an app got onto the machine, which decides how it should
// come off.
type Source string

const (
	SourceMAS      Source = "App Store"
	SourceCask     Source = "Homebrew"
	SourcePkg      Source = "instalador pkg"
	SourceSetapp   Source = "Setapp"
	SourceDragDrop Source = "arrastado"
	SourceUnknown  Source = "desconhecido"
)

// App is one installed application.
type App struct {
	Path       string     `json:"path"`
	Name       string     `json:"name"`
	BundleID   string     `json:"bundle_id"`
	Version    string     `json:"version"`
	Source     Source     `json:"source"`
	CaskToken  string     `json:"cask_token,omitempty"`
	PkgIDs     []string   `json:"pkg_ids,omitempty"`
	BundleSize int64      `json:"bundle_size"`
	Leftovers  []Leftover `json:"leftovers,omitempty"`
	Running    bool       `json:"running"`
}

// Footprint is the number that matters to someone freeing space: the bundle
// plus everything it left behind. Sorting by this, rather than by the size of
// the .app alone, is what makes the list worth reading.
func (a *App) Footprint() int64 {
	n := a.BundleSize
	for _, l := range a.Leftovers {
		n += l.Size
	}
	return n
}

// LeftoverSize is just the residue.
func (a *App) LeftoverSize() int64 {
	var n int64
	for _, l := range a.Leftovers {
		n += l.Size
	}
	return n
}

// Removable reports whether macsweep should offer to uninstall it at all.
func (a *App) Removable() bool { return a.Source != SourceSetapp }

// searchRoots are the places a user's own apps live. /System/Applications is
// deliberately absent: those belong to macOS and cannot be removed.
func searchRoots() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/Applications",
		"/Applications/Utilities",
		filepath.Join(home, "Applications"),
	}
}

// Discover finds installed apps, sizes them and matches their leftovers.
func Discover(ctx context.Context, opts Options) ([]*App, error) {
	// Read every Info.plist first. The bundle id table has to exist before
	// leftover matching starts, because the stop word list is derived from it.
	var apps []*App
	for _, path := range findBundles(searchRoots()) {
		if a := readBundle(path); a != nil {
			apps = append(apps, a)
		}
	}
	ids := make([]string, 0, len(apps))
	for _, a := range apps {
		ids = append(ids, a.BundleID)
	}

	index := newLeftoverIndex(opts, ids)
	running := runningBundles()
	casks := caskIndex()
	pkgs := pkgIndex()

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		out []*App
		sem = make(chan struct{}, 8)
	)

	for _, app := range apps {
		wg.Add(1)
		go func(app *App) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			app.Source, app.CaskToken, app.PkgIDs = classify(app, casks, pkgs)
			app.Running = running[app.BundleID] || running[app.Path]

			if res, err := scan.Walk(ctx, app.Path, scan.Options{}); err == nil {
				app.BundleSize = res.Root.Size
			}
			app.Leftovers = index.find(ctx, app)

			mu.Lock()
			out = append(out, app)
			mu.Unlock()
		}(app)
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool { return out[i].Footprint() > out[j].Footprint() })
	return out, nil
}

// findBundles lists .app directories one level down from each root, which
// covers /Applications, /Applications/Utilities and folders vendors create
// such as /Applications/Adobe.
func findBundles(roots []string) []string {
	seen := map[string]bool{}
	var out []string

	add := func(p string) {
		if strings.HasSuffix(p, ".app") && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(root, e.Name())
			if strings.HasSuffix(e.Name(), ".app") {
				add(p)
				continue
			}
			if !e.IsDir() {
				continue
			}
			nested, err := os.ReadDir(p)
			if err != nil {
				continue
			}
			for _, n := range nested {
				add(filepath.Join(p, n.Name()))
			}
		}
	}
	sort.Strings(out)
	return out
}

type infoPlist struct {
	BundleID    string `plist:"CFBundleIdentifier"`
	Name        string `plist:"CFBundleName"`
	DisplayName string `plist:"CFBundleDisplayName"`
	ShortVer    string `plist:"CFBundleShortVersionString"`
	Version     string `plist:"CFBundleVersion"`
}

// readBundle parses Info.plist. It has to handle both XML and binary plists,
// because system and vendor apps are split between the two formats.
func readBundle(path string) *App {
	f, err := os.Open(filepath.Join(path, "Contents", "Info.plist"))
	if err != nil {
		return nil
	}
	defer f.Close()

	var info infoPlist
	if err := plist.NewDecoder(f).Decode(&info); err != nil {
		return nil
	}
	if info.BundleID == "" {
		return nil
	}

	name := info.DisplayName
	if name == "" {
		name = info.Name
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(path), ".app")
	}
	version := info.ShortVer
	if version == "" {
		version = info.Version
	}

	return &App{Path: path, Name: name, BundleID: info.BundleID, Version: version}
}

func classify(a *App, casks map[string]string, pkgs map[string][]string) (Source, string, []string) {
	if strings.HasPrefix(a.Path, "/Applications/Setapp/") {
		return SourceSetapp, "", nil
	}
	if _, err := os.Stat(filepath.Join(a.Path, "Contents", "_MASReceipt", "receipt")); err == nil {
		return SourceMAS, "", nil
	}
	if token, ok := casks[strings.ToLower(a.Path)]; ok {
		return SourceCask, token, nil
	}
	if ids, ok := pkgs[a.Path]; ok {
		return SourcePkg, "", ids
	}
	return SourceDragDrop, "", nil
}

// caskIndex maps an app path to the Homebrew cask that owns it. A cask has a
// curated list of exactly what it installed, and `brew uninstall --cask`
// knows how to undo it, so deferring to brew beats any heuristic macsweep
// could write.
//
// Keys are lower cased because the receipt names an app as the cask spells
// it, which is often not how the bundle is spelled on disk.
func caskIndex() map[string]string {
	out := map[string]string{}
	prefix, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		return out
	}
	caskroom := filepath.Join(strings.TrimSpace(string(prefix)), "Caskroom")
	entries, err := os.ReadDir(caskroom)
	if err != nil {
		return out
	}

	roots := searchRoots()
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		token := e.Name()
		names := receiptApps(filepath.Join(caskroom, token, ".metadata", "INSTALL_RECEIPT.json"))
		// A cask whose receipt says nothing usually installs under its own
		// token, so that is the fallback rather than the first guess.
		names = append(names, token+".app")

		for _, name := range names {
			if filepath.IsAbs(name) {
				out[strings.ToLower(name)] = token
				continue
			}
			for _, root := range roots {
				candidate := filepath.Join(root, name)
				if _, err := os.Stat(candidate); err == nil {
					out[strings.ToLower(candidate)] = token
				}
			}
		}
	}
	return out
}

// receiptApps pulls the app bundle names out of a cask install receipt. The
// artifact list is loosely typed, mixing plain names with option objects, so
// it is decoded permissively and only the strings are kept.
func receiptApps(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var receipt struct {
		UninstallArtifacts []map[string][]any `json:"uninstall_artifacts"`
	}
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil
	}

	var out []string
	for _, artifact := range receipt.UninstallArtifacts {
		for _, entry := range artifact["app"] {
			if name, ok := entry.(string); ok && strings.HasSuffix(name, ".app") {
				out = append(out, name)
			}
		}
	}
	return out
}

// pkgIndex maps an app path to the receipt ids of the installers that put it
// there, so the receipts can be forgotten afterwards.
func pkgIndex() map[string][]string {
	out := map[string][]string{}
	list, err := exec.Command("pkgutil", "--pkgs").Output()
	if err != nil {
		return out
	}
	for _, id := range strings.Fields(string(list)) {
		loc, err := exec.Command("pkgutil", "--pkg-info", id).Output()
		if err != nil {
			continue
		}
		volume, location := "/", ""
		for _, line := range strings.Split(string(loc), "\n") {
			if v, ok := strings.CutPrefix(line, "location: "); ok {
				location = strings.TrimSpace(v)
			}
			if v, ok := strings.CutPrefix(line, "volume: "); ok {
				volume = strings.TrimSpace(v)
			}
		}
		if location == "" || !strings.HasSuffix(location, ".app") {
			continue
		}
		p := filepath.Join(volume, location)
		out[p] = append(out[p], id)
	}
	return out
}

// runningBundles indexes what is currently running, by bundle id and by path.
// Killing an app with unsaved work is data loss macsweep would have caused,
// so this is checked before anything is removed.
func runningBundles() map[string]bool {
	out := map[string]bool{}
	ps, err := exec.Command("ps", "-Ao", "args=").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(ps), "\n") {
		i := strings.Index(line, ".app/Contents/MacOS/")
		if i < 0 {
			continue
		}
		out[line[:i+4]] = true
	}
	return out
}
