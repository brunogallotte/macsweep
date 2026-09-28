// Package trash removes things reversibly.
//
// Two layers of undo, because neither is enough on its own. Items go to the
// macOS Trash, so a user who never reads the docs can still drag them back.
// And every batch writes a manifest, so `macsweep undo` restores exactly what
// a run removed. The manifest is the layer that actually holds: Finder's own
// Put Back is broken for every item after the first in a batch, an Apple bug
// open since 2015 (r.23153124), and users empty the Trash anyway.
package trash

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Item is one thing to remove.
type Item struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	// Label is what the user saw in the list, kept so undo output reads the
	// same way the removal did.
	Label string `json:"label,omitempty"`
}

// Outcome is what happened to one item.
type Outcome struct {
	Item
	TrashedTo string `json:"trashed_to,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (o Outcome) OK() bool { return o.Error == "" }

// Batch is one removal run, and the unit undo works on.
type Batch struct {
	ID        string    `json:"id"`
	Started   time.Time `json:"started"`
	Command   string    `json:"command"`
	Permanent bool      `json:"permanent"`
	// Purged means the Trash was emptied of this batch, so the space is
	// really back and undo has nothing left to restore.
	Purged   bool      `json:"purged,omitempty"`
	Outcomes []Outcome `json:"outcomes"`
}

// Reclaimed sums the sizes that were actually removed.
func (b *Batch) Reclaimed() int64 {
	var n int64
	for _, o := range b.Outcomes {
		if o.OK() {
			n += o.Size
		}
	}
	return n
}

// Failures returns the outcomes that did not go through.
func (b *Batch) Failures() []Outcome {
	var out []Outcome
	for _, o := range b.Outcomes {
		if !o.OK() {
			out = append(out, o)
		}
	}
	return out
}

// Options controls a removal.
type Options struct {
	// DryRun reports what would happen and touches nothing.
	DryRun bool
	// Permanent deletes outright instead of using the Trash. Only ever set
	// from an explicit flag, and refused for anything the Trash can take.
	Permanent bool
	// Command is recorded in the manifest, so undo can say what it is undoing.
	Command string
	// StateDir overrides where manifests are written. Empty means the
	// default under the user's state directory.
	StateDir string
}

// Send removes items and writes a manifest. It never stops on the first
// failure: a batch is a list of independent removals, and one refusal should
// not hide the rest.
func Send(items []Item, opts Options) (*Batch, error) {
	b := &Batch{
		ID:        time.Now().UTC().Format("20060102-150405"),
		Started:   time.Now(),
		Command:   opts.Command,
		Permanent: opts.Permanent,
	}

	for _, it := range items {
		out := Outcome{Item: it}
		switch {
		case opts.DryRun:
			out.TrashedTo = "(simulacao)"
		case opts.Permanent:
			if err := os.RemoveAll(it.Path); err != nil {
				out.Error = err.Error()
			}
		default:
			dest, err := trashItem(it.Path)
			if err != nil {
				out.Error = err.Error()
			} else {
				out.TrashedTo = dest
			}
		}
		b.Outcomes = append(b.Outcomes, out)
	}

	if opts.DryRun {
		return b, nil
	}
	return b, writeManifest(b, opts.StateDir)
}

// StateDir is where manifests live.
func StateDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "macsweep", "undo"), nil
}

func writeManifest(b *Batch, override string) error {
	dir, err := StateDir(override)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, b.ID+".json"), data, 0o600)
}

// History lists past batches, newest first.
func History(override string) ([]*Batch, error) {
	dir, err := StateDir(override)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []*Batch
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var b Batch
		if json.Unmarshal(data, &b) == nil {
			out = append(out, &b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out, nil
}

// Undo puts a batch back where it came from. A permanent batch cannot be
// undone, and says so rather than pretending.
func Undo(id, override string) (restored int, err error) {
	batches, err := History(override)
	if err != nil {
		return 0, err
	}

	var target *Batch
	for _, b := range batches {
		if b.ID == id || id == "" {
			target = b
			break
		}
	}
	if target == nil {
		return 0, fmt.Errorf("nenhuma operacao encontrada com id %q", id)
	}
	if target.Permanent {
		return 0, fmt.Errorf("a operacao %s foi permanente e nao pode ser desfeita", target.ID)
	}
	if target.Purged {
		return 0, fmt.Errorf("a Lixeira ja foi esvaziada para a operacao %s", target.ID)
	}

	var firstErr error
	for _, o := range target.Outcomes {
		if !o.OK() || o.TrashedTo == "" {
			continue
		}
		if _, statErr := os.Stat(o.TrashedTo); statErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s nao esta mais na Lixeira", filepath.Base(o.Path))
			}
			continue
		}
		if _, statErr := os.Stat(o.Path); statErr == nil {
			continue // something is already there, do not overwrite it
		}
		if mkErr := os.MkdirAll(filepath.Dir(o.Path), 0o755); mkErr != nil {
			if firstErr == nil {
				firstErr = mkErr
			}
			continue
		}
		if mvErr := os.Rename(o.TrashedTo, o.Path); mvErr != nil {
			if firstErr == nil {
				firstErr = mvErr
			}
			continue
		}
		restored++
	}
	return restored, firstErr
}
