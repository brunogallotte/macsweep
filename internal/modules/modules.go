// Package modules holds what each macsweep module actually does, independent
// of how it is driven. The one shot subcommands and the interactive app both
// run the same pipeline through here, so the two can never drift apart.
package modules

import (
	"context"

	"github.com/brunogallotte/macsweep/internal/rules"
	"github.com/brunogallotte/macsweep/internal/safety"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// Env is everything a module needs to know about the machine and the user's
// choices.
type Env struct {
	Home  string
	Guard *safety.Guard

	// Roots limits the project scan. Empty means the default roots.
	Roots []string
	// IdleDays decides which project artifacts come preselected.
	IdleDays int
	// Deep turns on name based leftover matching in the apps module.
	Deep bool
	// Risk caps which cleaning rules are offered.
	Risk rules.Risk
	// Zap uses Homebrew's full removal list for casks.
	Zap bool

	// Progress, when set, is called with a short status line during a scan.
	Progress func(string)
}

func (e *Env) say(msg string) {
	if e.Progress != nil {
		e.Progress(msg)
	}
}

// Command is an action delegated to the tool that owns the data, because only
// docker knows how to reclaim docker space and only brew knows everything a
// cask installed.
type Command struct {
	Title string
	Argv  []string
}

// Result is what a scan produced and how to act on it.
type Result struct {
	Title    string
	Subtitle string
	Rows     []ui.Row

	// Expand turns the user's selection into the concrete things to remove.
	// The apps module uses it to go from "these four apps" to "these
	// forty-one paths". nil means the rows are already the items.
	Expand func(chosen []ui.Row) ([]ui.Row, []Command)

	// Preview asks the front end for a second confirmation pass over the
	// expanded items, showing every path before anything moves.
	Preview bool
}

// Empty reports whether there is nothing to act on.
func (r Result) Empty() bool { return len(r.Rows) == 0 }

// Total is everything the scan found.
func (r Result) Total() int64 {
	var n int64
	for _, row := range r.Rows {
		n += row.Size
	}
	return n
}

// Kind identifies a module.
type Kind string

const (
	KindApps     Kind = "apps"
	KindClean    Kind = "clean"
	KindProjects Kind = "projects"
	KindEmpty    Kind = "empty"
)

// Scan runs the module named by kind.
func Scan(ctx context.Context, kind Kind, env *Env) (Result, error) {
	switch kind {
	case KindApps:
		return ScanApps(ctx, env)
	case KindClean:
		return ScanClean(ctx, env)
	case KindProjects:
		return ScanProjects(ctx, env)
	case KindEmpty:
		return ScanEmpty(ctx, env)
	}
	return Result{}, nil
}
