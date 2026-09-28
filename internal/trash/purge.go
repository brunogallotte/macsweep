package trash

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Moving something to the Trash frees no space at all: it is a rename inside
// the same volume. The bytes come back only when the Trash is emptied, which
// is why macsweep tracks what it put there and can purge exactly that,
// leaving anything the user trashed by hand alone.

// Pending is one item macsweep moved to the Trash that is still sitting there.
type Pending struct {
	BatchID string
	Path    string // where it came from
	Trashed string // where it is now
	Size    int64
	Label   string
}

// PendingItems lists everything macsweep trashed that has not been emptied.
//
// It reads the manifests rather than the Trash directory, because listing
// ~/.Trash needs Full Disk Access while checking a known path inside it does
// not.
func PendingItems(stateDir string) ([]Pending, int64, error) {
	batches, err := History(stateDir)
	if err != nil {
		return nil, 0, err
	}

	var out []Pending
	var total int64
	for _, b := range batches {
		if b.Permanent || b.Purged {
			continue
		}
		for _, o := range b.Outcomes {
			if !o.OK() || o.TrashedTo == "" {
				continue
			}
			if _, err := os.Stat(o.TrashedTo); err != nil {
				continue // already gone, emptied by hand
			}
			out = append(out, Pending{
				BatchID: b.ID, Path: o.Path, Trashed: o.TrashedTo,
				Size: o.Size, Label: o.Label,
			})
			total += o.Size
		}
	}
	return out, total, nil
}

// PurgeReport is the outcome of emptying.
type PurgeReport struct {
	Items    int
	Size     int64 // what the manifests said those items weighed
	Failures []string
}

// Purge permanently removes the given items and marks their batches as
// emptied, so undo stops offering to restore what no longer exists.
func Purge(stateDir string, items []Pending) PurgeReport {
	var rep PurgeReport
	touched := map[string]bool{}

	for _, it := range items {
		if err := os.RemoveAll(it.Trashed); err != nil {
			rep.Failures = append(rep.Failures, it.Trashed+": "+err.Error())
			continue
		}
		rep.Items++
		rep.Size += it.Size
		touched[it.BatchID] = true
	}

	for id := range touched {
		markPurged(stateDir, id)
	}
	return rep
}

func markPurged(stateDir, id string) {
	dir, err := StateDir(stateDir)
	if err != nil {
		return
	}
	path := filepath.Join(dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var b Batch
	if json.Unmarshal(data, &b) != nil {
		return
	}
	b.Purged = true
	if out, err := json.MarshalIndent(&b, "", "  "); err == nil {
		_ = os.WriteFile(path, out, 0o600)
	}
}
