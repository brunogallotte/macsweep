package scan

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWalkAggregates(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a", "um.bin"), 8192)
	write(t, filepath.Join(dir, "a", "dois.bin"), 8192)
	write(t, filepath.Join(dir, "b", "tres.bin"), 4096)

	res, err := Walk(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stats.Files != 3 {
		t.Errorf("esperado 3 arquivos, obtido %d", res.Stats.Files)
	}

	res.Root.SortBySize()
	if got := res.Root.Children[0].Name; got != "a" {
		t.Errorf("o maior diretorio deveria vir primeiro, veio %q", got)
	}
	if res.Root.Size < 20480 {
		t.Errorf("total agregado baixo demais: %d", res.Root.Size)
	}
}

// A hard link is one set of bytes with two names. Counting it twice is how a
// cleaner promises space it cannot deliver.
func TestHardLinkCountedOnce(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.bin")
	write(t, original, 65536)

	if err := os.Link(original, filepath.Join(dir, "link.bin")); err != nil {
		t.Skipf("sistema de arquivos sem hard link: %v", err)
	}

	res, err := Walk(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Room for the directory entry itself, but nowhere near twice the file.
	if res.Root.Size > 100000 {
		t.Errorf("hard link contado duas vezes: %d bytes para um arquivo de 64 KB", res.Root.Size)
	}
}

// A sparse file reserves far fewer blocks than its logical length. Docker.raw
// is the one that matters here: it reports tens of GB it does not occupy.
func TestSparseFileMeasuredByBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "esparso.bin")

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// One gigabyte of logical length, one byte of content.
	if err := f.Truncate(1 << 30); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 1<<30 {
		t.Fatalf("tamanho logico inesperado: %d", fi.Size())
	}
	if allocated := Allocated(fi); allocated >= 1<<29 {
		t.Errorf("arquivo esparso medido pelo tamanho logico: %d bytes alocados", allocated)
	}
}

// A symlink is measured as a link and never followed, so a loop cannot hang
// the walk and a link into a huge tree cannot inflate the total.
func TestSymlinkNotFollowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "alvo")
	write(t, filepath.Join(target, "grande.bin"), 1<<20)

	inner := filepath.Join(dir, "dentro")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(inner, "atalho")); err != nil {
		t.Skip("sem symlink neste ambiente")
	}

	res, err := Walk(context.Background(), inner, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Root.Size > 1<<16 {
		t.Errorf("symlink foi seguido: %d bytes", res.Root.Size)
	}
}

// The whole point of the package: agree with du, which is the number the user
// can verify themselves.
func TestAgreesWithDu(t *testing.T) {
	dir := t.TempDir()
	for i, size := range []int{4096, 65536, 1 << 20} {
		write(t, filepath.Join(dir, "sub", string(rune('a'+i))+".bin"), size)
	}

	out, err := exec.Command("du", "-skx", dir).Output()
	if err != nil {
		t.Skip("du indisponivel")
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		t.Skip("saida do du inesperada")
	}
	kb, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		t.Skip("saida do du inesperada")
	}

	res, err := Walk(context.Background(), dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := kb * 1024
	diff := res.Root.Size - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 8192 {
		t.Errorf("divergencia com du: macsweep %d, du %d", res.Root.Size, want)
	}
}
