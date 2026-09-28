package ui

import (
	"fmt"
	"strings"
)

// Outcome is everything worth saying after an operation. It exists as data
// rather than as print statements so the one shot commands and the
// interactive app report the same way.
type Outcome struct {
	Command   string
	DryRun    bool
	Permanent bool

	Items  int
	Failed int
	// Size is what the removed items weighed.
	Size int64

	FreeBefore int64
	FreeAfter  int64
	DiskTotal  int64

	// InTrash is everything macsweep has sitting in the Trash after this
	// operation, including earlier runs. It is the number `macsweep empty`
	// would actually give back.
	InTrash int64

	BatchID  string
	Failures []string
}

// Gain is the space genuinely returned to the disk.
func (o Outcome) Gain() int64 {
	if g := o.FreeAfter - o.FreeBefore; g > 0 {
		return g
	}
	return 0
}

// Render writes the report.
func (t *Theme) Render(o Outcome) string {
	var b strings.Builder

	if o.DryRun {
		fmt.Fprintf(&b, "\n  %s  %s em %d itens\n",
			t.Warn.Render("simulacao"), t.Size.Render(Size(o.Size)), o.Items)
		b.WriteString(t.Subtle.Render("  nada foi removido. rode sem --dry-run para valer.\n"))
		return b.String()
	}

	if o.Items == 0 && o.Failed == 0 {
		return t.Subtle.Render("\n  nada foi removido\n")
	}

	verb := "para a Lixeira"
	if o.Permanent {
		verb = "apagados"
	}
	fmt.Fprintf(&b, "\n  %s %d itens %s%s%s\n",
		t.Good.Render("✓"), o.Items, verb,
		strings.Repeat(" ", max(34-len(verb)-numLen(o.Items), 2)),
		t.Size.Render(Size(o.Size)))

	b.WriteString("\n")
	b.WriteString(t.spaceLine(o))

	switch {
	case o.Permanent && o.Gain() == 0 && o.Size > 1<<30:
		// Space really was released but the filesystem does not show it yet.
		// On APFS a local snapshot pins the blocks of anything deleted, often
		// for hours.
		b.WriteString("\n  " + t.Warn.Render("!") + " " + t.Subtle.Render(
			"o disco ainda nao mostra o ganho. no APFS um snapshot local costuma\n"+
				"    segurar os blocos por algumas horas. veja `macsweep doctor`.") + "\n")

	case !o.Permanent:
		// The honest part: a move to the Trash frees nothing on its own.
		b.WriteString("\n  " + t.Warn.Render("!") + " " + t.Subtle.Render(
			"a Lixeira ainda guarda "+Size(o.InTrash)+". o espaco so volta\n"+
				"    quando ela for esvaziada.") + "\n")
		fmt.Fprintf(&b, "\n    %s  %s\n",
			t.Accent.Render(fmt.Sprintf("%-18s", "macsweep empty")),
			t.Subtle.Render("libera "+Size(o.InTrash)+" agora"))
		fmt.Fprintf(&b, "    %s  %s\n",
			t.Accent.Render(fmt.Sprintf("%-18s", "macsweep undo")),
			t.Subtle.Render("devolve tudo para o lugar (operacao "+o.BatchID+")"))
	}

	if o.Failed > 0 {
		fmt.Fprintf(&b, "\n  %s %d itens nao puderam ser removidos:\n",
			t.Warn.Render("atencao:"), o.Failed)
		for i, f := range o.Failures {
			if i >= 5 {
				fmt.Fprintf(&b, "    %s\n", t.Subtle.Render(
					fmt.Sprintf("e mais %d", len(o.Failures)-5)))
				break
			}
			fmt.Fprintf(&b, "    %s\n", t.Subtle.Render(f))
		}
	}
	return b.String()
}

// spaceLine shows free space before and after, with the gain called out when
// there is one. This is the comparison the user asked to see.
func (t *Theme) spaceLine(o Outcome) string {
	var b strings.Builder

	fmt.Fprintf(&b, "    %s %s %s %s\n",
		t.Subtle.Render("espaco livre "),
		t.Subtle.Render(fmt.Sprintf("%9s", Size(o.FreeBefore))),
		t.Subtle.Render("→"),
		t.Size.Render(fmt.Sprintf("%9s", Size(o.FreeAfter))))

	if gain := o.Gain(); gain > 0 {
		fmt.Fprintf(&b, "    %s %s  %s\n",
			t.Subtle.Render("ganho        "),
			t.Good.Render(fmt.Sprintf("%9s", "+"+Size(gain))),
			t.SizeBar(gain, max64(o.DiskTotal/4, gain), 20))
	}
	return b.String()
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func numLen(n int) int { return len(fmt.Sprint(n)) }
