package ui

import (
	"strings"
	"testing"
)

const gb = 1 << 30

// Moving to the Trash frees nothing, and the report must say so instead of
// claiming a gain it did not produce.
func TestTrashReportDoesNotClaimAGain(t *testing.T) {
	out := Outcome{
		Command: "projects", Items: 315, Size: 25 * gb,
		FreeBefore: 12 * gb, FreeAfter: 12 * gb, DiskTotal: 228 * gb,
		InTrash: 37 * gb, BatchID: "20260921-170230",
	}
	got := NewPlainTheme().Render(out)

	if strings.Contains(got, "ganho") {
		t.Error("nao pode anunciar ganho quando nada foi liberado")
	}
	for _, want := range []string{"315 itens para a Lixeira", "macsweep empty", "macsweep undo", "20260921-170230"} {
		if !strings.Contains(got, want) {
			t.Errorf("faltou %q em:\n%s", want, got)
		}
	}
	if strings.Contains(got, "snapshot") {
		t.Error("nao pode culpar snapshot: os arquivos estao na Lixeira")
	}
}

// Emptying is the operation that really returns space, so it shows the gain.
func TestPurgeReportShowsTheGain(t *testing.T) {
	out := Outcome{
		Command: "empty", Permanent: true, Items: 315, Size: 25 * gb,
		FreeBefore: 12 * gb, FreeAfter: 37 * gb, DiskTotal: 228 * gb,
	}
	got := NewPlainTheme().Render(out)

	if !strings.Contains(got, "ganho") || !strings.Contains(got, "+25.0 GB") {
		t.Errorf("deveria mostrar o ganho medido:\n%s", got)
	}
	if !strings.Contains(got, "12.0 GB") || !strings.Contains(got, "37.0 GB") {
		t.Errorf("deveria mostrar antes e depois:\n%s", got)
	}
}

// A permanent delete that frees nothing is where a local snapshot really is
// the likely explanation.
func TestPermanentWithoutGainBlamesSnapshots(t *testing.T) {
	out := Outcome{
		Command: "clean", Permanent: true, Items: 10, Size: 5 * gb,
		FreeBefore: 12 * gb, FreeAfter: 12 * gb, DiskTotal: 228 * gb,
	}
	got := NewPlainTheme().Render(out)
	if !strings.Contains(got, "snapshot") {
		t.Errorf("deveria explicar o snapshot:\n%s", got)
	}
}

func TestDryRunSaysNothingHappened(t *testing.T) {
	got := NewPlainTheme().Render(Outcome{DryRun: true, Items: 5, Size: 2 * gb})
	if !strings.Contains(got, "simulacao") || !strings.Contains(got, "nada foi removido") {
		t.Errorf("simulacao mal reportada:\n%s", got)
	}
}
