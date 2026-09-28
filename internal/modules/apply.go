package modules

import (
	"os/exec"
	"strings"

	"github.com/brunogallotte/macsweep/internal/scan"
	"github.com/brunogallotte/macsweep/internal/trash"
	"github.com/brunogallotte/macsweep/internal/ui"
)

// ApplyOptions controls how a selection is carried out.
type ApplyOptions struct {
	Command   string
	Home      string
	StateDir  string
	DryRun    bool
	Permanent bool
	// Purge means these items are already in the Trash and are being emptied
	// for good, which is the only operation that really returns space.
	Purge bool
}

// Apply carries out a selection and reports what actually happened.
func Apply(items []ui.Row, cmds []Command, opts ApplyOptions) ui.Outcome {
	before, _ := scan.VolumeAt(opts.Home)

	out := ui.Outcome{
		Command:   opts.Command,
		DryRun:    opts.DryRun,
		Permanent: opts.Permanent || opts.Purge,
		DiskTotal: before.Total,
	}

	// Tool owned cleanups run first: they are the ones that can take a while
	// and the ones whose failure should not stop the file removals.
	for _, c := range cmds {
		if opts.DryRun {
			out.Items++
			continue
		}
		if o, err := exec.Command(c.Argv[0], c.Argv[1:]...).CombinedOutput(); err != nil {
			out.Failed++
			out.Failures = append(out.Failures, c.Title+": "+firstLine(string(o)))
		} else {
			out.Items++
		}
	}

	switch {
	case opts.Purge:
		out = applyPurge(items, opts, out)
	default:
		out = applyRemove(items, opts, out)
	}

	after, _ := scan.VolumeAt(opts.Home)
	out.FreeBefore, out.FreeAfter = before.Free, after.Free

	if _, inTrash, err := trash.PendingItems(opts.StateDir); err == nil {
		out.InTrash = inTrash
	}
	return out
}

func applyRemove(items []ui.Row, opts ApplyOptions, out ui.Outcome) ui.Outcome {
	if len(items) == 0 {
		return out
	}

	list := make([]trash.Item, 0, len(items))
	for _, r := range items {
		list = append(list, trash.Item{Path: r.Detail, Size: r.Size, Label: r.Title})
	}

	batch, err := trash.Send(list, trash.Options{
		DryRun:    opts.DryRun,
		Permanent: opts.Permanent,
		Command:   opts.Command,
		StateDir:  opts.StateDir,
	})
	if err != nil {
		out.Failed++
		out.Failures = append(out.Failures, err.Error())
		return out
	}

	out.BatchID = batch.ID
	out.Size += batch.Reclaimed()
	out.Items += len(list) - len(batch.Failures())
	for _, f := range batch.Failures() {
		out.Failed++
		out.Failures = append(out.Failures, f.Label+": "+f.Error)
	}
	return out
}

func applyPurge(items []ui.Row, opts ApplyOptions, out ui.Outcome) ui.Outcome {
	pending, _, err := trash.PendingItems(opts.StateDir)
	if err != nil {
		out.Failures = append(out.Failures, err.Error())
		out.Failed++
		return out
	}

	keep := make(map[string]bool, len(items))
	for _, r := range items {
		keep[r.ID] = true
	}
	var selected []trash.Pending
	for _, p := range pending {
		if keep[p.Trashed] {
			selected = append(selected, p)
		}
	}

	if opts.DryRun {
		for _, p := range selected {
			out.Size += p.Size
			out.Items++
		}
		return out
	}

	rep := trash.Purge(opts.StateDir, selected)
	out.Items += rep.Items
	out.Size += rep.Size
	out.Failed += len(rep.Failures)
	out.Failures = append(out.Failures, rep.Failures...)
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	if len(s) > 120 {
		return s[:120]
	}
	return s
}
