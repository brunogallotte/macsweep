package projects

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestAcceptance runs the discovery against a real directory and prints what
// it found. It is opt in because it depends on the machine it runs on:
//
//	MACSWEEP_ACCEPTANCE=$HOME/Documents go test ./internal/projects/ -run Acceptance -v
func TestAcceptance(t *testing.T) {
	root := os.Getenv("MACSWEEP_ACCEPTANCE")
	if root == "" {
		t.Skip("defina MACSWEEP_ACCEPTANCE com o diretorio a varrer")
	}

	start := time.Now()
	ps, stats, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	var total int64
	var artifacts int
	for _, p := range ps {
		total += p.Size()
		artifacts += len(p.Artifacts)
	}

	t.Logf("%d projetos, %d artefatos, %s, em %s (negados %d)",
		len(ps), artifacts, gb(total), time.Since(start).Round(time.Millisecond), stats.Denied)

	for i, p := range ps {
		if i >= 10 {
			break
		}
		t.Logf("%10s  %-30s ocioso ha %.0fd, %d artefatos",
			gb(p.Size()), p.Name, p.Idle().Hours()/24, len(p.Artifacts))
		for j, a := range p.Artifacts {
			if j >= 2 {
				break
			}
			t.Logf("            %10s  %-14s %s", gb(a.Size), a.Kind, a.Regenerate)
		}
	}
}

func gb(b int64) string { return fmt.Sprintf("%.2f GB", float64(b)/(1<<30)) }
