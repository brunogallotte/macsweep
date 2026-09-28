// Package projects finds regenerable build output inside code repositories.
//
// This is where a developer's disk actually goes. The existing cleaners look
// for it only under a fixed set of folder names (~/Projects, ~/Code, ~/dev,
// ~/GitHub, ~/Workspace), which misses anyone who keeps repositories
// somewhere else. macsweep scans configured roots and defaults to including
// ~/Documents, because that is where a lot of people actually work.
package projects

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/brunogallotte/macsweep/internal/scan"
)

// Kind describes one flavour of build output.
type Kind struct {
	// Dir is the directory name that identifies it.
	Dir string
	// Markers are files that must exist in the same directory for a match to
	// count. This is what keeps a source folder named bin, dist or build from
	// being mistaken for build output: without a project manifest next to it,
	// it is just a folder.
	Markers []string
	// Regenerate is the command that brings it back.
	Regenerate string
	// Tool names the ecosystem, for grouping in the UI.
	Tool string
}

// kinds is deliberately conservative. Every entry that shares its name with a
// plausible source directory carries markers.
var kinds = []Kind{
	{Dir: "node_modules", Markers: []string{"package.json"}, Regenerate: "npm install", Tool: "node"},
	{Dir: ".next", Markers: []string{"package.json"}, Regenerate: "next build", Tool: "node"},
	{Dir: ".nuxt", Markers: []string{"package.json"}, Regenerate: "nuxt build", Tool: "node"},
	{Dir: ".turbo", Markers: []string{"package.json"}, Regenerate: "turbo build", Tool: "node"},
	{Dir: ".svelte-kit", Markers: []string{"package.json"}, Regenerate: "vite build", Tool: "node"},
	{Dir: ".angular", Markers: []string{"angular.json"}, Regenerate: "ng build", Tool: "node"},
	{Dir: ".parcel-cache", Markers: []string{"package.json"}, Regenerate: "parcel build", Tool: "node"},
	{Dir: "dist", Markers: []string{"package.json"}, Regenerate: "npm run build", Tool: "node"},

	{Dir: "bin", Markers: []string{"*.csproj", "*.fsproj", "*.sln"}, Regenerate: "dotnet build", Tool: "dotnet"},
	{Dir: "obj", Markers: []string{"*.csproj", "*.fsproj", "*.sln"}, Regenerate: "dotnet restore", Tool: "dotnet"},

	{Dir: "target", Markers: []string{"Cargo.toml"}, Regenerate: "cargo build", Tool: "rust"},
	{Dir: "target", Markers: []string{"pom.xml"}, Regenerate: "mvn package", Tool: "java"},
	{Dir: "build", Markers: []string{"build.gradle", "build.gradle.kts"}, Regenerate: "gradle build", Tool: "java"},
	{Dir: ".gradle", Markers: []string{"build.gradle", "build.gradle.kts", "settings.gradle"}, Regenerate: "gradle build", Tool: "java"},

	{Dir: ".venv", Markers: []string{"pyproject.toml", "requirements.txt", "setup.py"}, Regenerate: "uv sync", Tool: "python"},
	{Dir: "venv", Markers: []string{"pyproject.toml", "requirements.txt", "setup.py"}, Regenerate: "python -m venv venv", Tool: "python"},
	{Dir: "__pycache__", Regenerate: "regenerado automaticamente", Tool: "python"},

	{Dir: "Pods", Markers: []string{"Podfile"}, Regenerate: "pod install", Tool: "swift"},
	{Dir: "DerivedData", Markers: []string{"*.xcodeproj", "*.xcworkspace"}, Regenerate: "xcodebuild", Tool: "swift"},
	{Dir: ".build", Markers: []string{"Package.swift"}, Regenerate: "swift build", Tool: "swift"},

	{Dir: "vendor", Markers: []string{"go.mod"}, Regenerate: "go mod vendor", Tool: "go"},
	{Dir: "vendor", Markers: []string{"composer.json"}, Regenerate: "composer install", Tool: "php"},

	{Dir: "cmake-build-debug", Markers: []string{"CMakeLists.txt"}, Regenerate: "cmake --build", Tool: "cpp"},
	{Dir: "cmake-build-release", Markers: []string{"CMakeLists.txt"}, Regenerate: "cmake --build", Tool: "cpp"},
}

// Artifact is one regenerable directory.
type Artifact struct {
	Path       string    `json:"path"`
	Kind       string    `json:"kind"`
	Tool       string    `json:"tool"`
	Size       int64     `json:"size"`
	Modified   time.Time `json:"modified"`
	Regenerate string    `json:"regenerate"`
}

// Idle reports how long since the artifact was last written to.
func (a Artifact) Idle() time.Duration { return time.Since(a.Modified) }

// Project groups the artifacts found under one repository.
type Project struct {
	Root      string     `json:"root"`
	Name      string     `json:"name"`
	LastGit   time.Time  `json:"last_git,omitempty"`
	Artifacts []Artifact `json:"artifacts"`
}

// Size is everything reclaimable in this project.
func (p *Project) Size() int64 {
	var n int64
	for _, a := range p.Artifacts {
		n += a.Size
	}
	return n
}

// Idle is the freshest artifact's age, which is the honest measure of whether
// a project is still being worked on.
func (p *Project) Idle() time.Duration {
	newest := time.Time{}
	for _, a := range p.Artifacts {
		if a.Modified.After(newest) {
			newest = a.Modified
		}
	}
	if !p.LastGit.IsZero() && p.LastGit.After(newest) {
		newest = p.LastGit
	}
	if newest.IsZero() {
		return 0
	}
	return time.Since(newest)
}

// Discover walks root and returns the projects found beneath it.
func Discover(ctx context.Context, root string) ([]*Project, scan.Stats, error) {
	res, err := scan.Walk(ctx, root, scan.Options{})
	if err != nil {
		return nil, scan.Stats{}, err
	}
	return FromTree(res.Root), res.Stats, nil
}

// FromTree extracts projects from an already scanned tree. Keeping this
// separate from the walk means the panel can scan once and feed every module
// from the same tree, and it makes the matching testable without a disk.
func FromTree(root *scan.Node) []*Project {
	byRoot := map[string]*Project{}

	root.Walk(func(n *scan.Node) bool {
		if !n.IsDir || n.Parent == nil {
			return true
		}
		kind, ok := match(n)
		if !ok {
			return true
		}

		projectRoot := findProjectRoot(n.Parent)
		p := byRoot[projectRoot]
		if p == nil {
			p = &Project{
				Root:    projectRoot,
				Name:    filepath.Base(projectRoot),
				LastGit: gitActivity(projectRoot),
			}
			byRoot[projectRoot] = p
		}
		p.Artifacts = append(p.Artifacts, Artifact{
			Path:       n.Path,
			Kind:       n.Name,
			Tool:       kind.Tool,
			Size:       n.Size,
			Modified:   n.ModTime,
			Regenerate: kind.Regenerate,
		})

		// Nothing inside build output is interesting, and a nested
		// node_modules would be double counted.
		return false
	})

	out := make([]*Project, 0, len(byRoot))
	for _, p := range byRoot {
		sort.Slice(p.Artifacts, func(i, j int) bool { return p.Artifacts[i].Size > p.Artifacts[j].Size })
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size() > out[j].Size() })
	return out
}

// match reports whether a directory is build output, requiring the marker
// files of its kind to sit beside it.
func match(n *scan.Node) (Kind, bool) {
	for _, k := range kinds {
		if k.Dir != n.Name {
			continue
		}
		if len(k.Markers) == 0 || hasMarker(n.Parent, k.Markers) {
			return k, true
		}
	}
	return Kind{}, false
}

func hasMarker(parent *scan.Node, markers []string) bool {
	if parent == nil {
		return false
	}
	for _, c := range parent.Children {
		if c.IsDir {
			continue
		}
		for _, m := range markers {
			if ok, _ := filepath.Match(m, c.Name); ok {
				return true
			}
		}
	}
	return false
}

// findProjectRoot walks up to the nearest repository root, falling back to
// the directory that held the artifact.
func findProjectRoot(n *scan.Node) string {
	for cur := n; cur != nil; cur = cur.Parent {
		for _, c := range cur.Children {
			if c.IsDir && c.Name == ".git" {
				return cur.Path
			}
		}
	}
	return n.Path
}

// gitActivity uses the mtime of .git/index as a cheap stand-in for the last
// commit, avoiding a dependency on git being installed.
func gitActivity(root string) time.Time {
	fi, err := os.Stat(filepath.Join(root, ".git", "index"))
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}
