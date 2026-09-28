package trash

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDryRunTouchesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arquivo.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := Send([]Item{{Path: path, Size: 1}}, Options{DryRun: true, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Failures()) != 0 {
		t.Fatalf("simulacao nao deveria falhar: %v", b.Failures())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("simulacao apagou o arquivo")
	}
}

// The round trip that the whole safety story rests on: remove, then restore
// from the manifest.
func TestTrashAndUndoRoundTrip(t *testing.T) {
	if os.Getenv("MACSWEEP_TRASH_TEST") == "" {
		t.Skip("define MACSWEEP_TRASH_TEST=1 para exercitar a Lixeira de verdade")
	}

	state := t.TempDir()
	dir := t.TempDir()
	path := filepath.Join(dir, "macsweep-roundtrip.txt")
	want := []byte("conteudo original")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := Send([]Item{{Path: path, Size: int64(len(want))}},
		Options{Command: "teste", StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	if f := b.Failures(); len(f) > 0 {
		t.Fatalf("remocao falhou: %v", f)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("arquivo continua no lugar depois da remocao")
	}

	n, err := Undo(b.ID, state)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperado 1 restaurado, obtido %d", n)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("arquivo nao voltou: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("conteudo mudou: %q", got)
	}
}

func TestUndoRefusesPermanentBatch(t *testing.T) {
	state := t.TempDir()
	dir := t.TempDir()
	path := filepath.Join(dir, "some.txt")
	os.WriteFile(path, []byte("x"), 0o644)

	b, err := Send([]Item{{Path: path}}, Options{Permanent: true, StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Undo(b.ID, state); err == nil {
		t.Fatal("undo de operacao permanente deveria falhar explicitamente")
	}
}
