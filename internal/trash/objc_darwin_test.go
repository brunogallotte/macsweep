//go:build darwin

package trash

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTrashItem really moves a file to the Trash, so it is opt in. Run it
// with MACSWEEP_TRASH_TEST=1 go test ./internal/trash/ -run TrashItem -v
func TestTrashItem(t *testing.T) {
	if os.Getenv("MACSWEEP_TRASH_TEST") == "" {
		t.Skip("define MACSWEEP_TRASH_TEST=1 para exercitar a Lixeira de verdade")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "macsweep-spike.txt")
	if err := os.WriteFile(path, []byte("descartavel"), 0o644); err != nil {
		t.Fatal(err)
	}

	landed, err := trashItem(path)
	if err != nil {
		t.Fatalf("trashItem: %v", err)
	}
	if landed == "" {
		t.Fatal("trashItem nao devolveu o caminho de destino")
	}
	t.Logf("foi parar em: %s", landed)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("original ainda existe em %s", path)
	}
	if _, err := os.Stat(landed); err != nil {
		t.Fatalf("destino nao existe: %v", err)
	}
}
