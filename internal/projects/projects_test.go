package projects

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func mkfile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func discover(t *testing.T, root string) []*Project {
	t.Helper()
	ps, _, err := Discover(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return ps
}

func artifactKinds(ps []*Project) map[string]bool {
	out := map[string]bool{}
	for _, p := range ps {
		for _, a := range p.Artifacts {
			out[a.Kind] = true
		}
	}
	return out
}

func TestFindsBuildOutput(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	mkfile(t, filepath.Join(app, "package.json"), 32)
	mkfile(t, filepath.Join(app, "node_modules", "dep", "index.js"), 4096)
	mkfile(t, filepath.Join(app, ".next", "build.js"), 8192)

	ps := discover(t, root)
	if len(ps) != 1 {
		t.Fatalf("esperado 1 projeto, obtido %d", len(ps))
	}
	k := artifactKinds(ps)
	if !k["node_modules"] || !k[".next"] {
		t.Fatalf("artefatos nao encontrados: %v", k)
	}
	if ps[0].Size() == 0 {
		t.Error("projeto sem tamanho")
	}
}

// The rule that keeps this tool from deleting people's source code: a folder
// named bin, dist or build is only build output when a project manifest sits
// beside it.
func TestMarkerRequiredBeforeDeleting(t *testing.T) {
	root := t.TempDir()

	// A plain folder of scripts that happens to be called bin.
	mkfile(t, filepath.Join(root, "scripts", "bin", "deploy.sh"), 1024)
	// A folder called dist with no package.json anywhere near it.
	mkfile(t, filepath.Join(root, "assets", "dist", "logo.svg"), 1024)

	if ps := discover(t, root); len(ps) != 0 {
		t.Fatalf("pastas sem manifesto nao podem contar como artefato: %+v", ps[0].Artifacts)
	}

	// The same name, now with a manifest beside it.
	proj := filepath.Join(root, "servico")
	mkfile(t, filepath.Join(proj, "Servico.csproj"), 64)
	mkfile(t, filepath.Join(proj, "bin", "Debug", "app.dll"), 4096)

	if k := artifactKinds(discover(t, root)); !k["bin"] {
		t.Error("bin ao lado de um .csproj deveria contar como artefato")
	}
}

// Nothing inside build output is interesting on its own, and a nested
// node_modules would be counted twice.
func TestDoesNotDescendIntoArtifacts(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	mkfile(t, filepath.Join(app, "package.json"), 32)
	mkfile(t, filepath.Join(app, "node_modules", "pacote", "package.json"), 32)
	mkfile(t, filepath.Join(app, "node_modules", "pacote", "node_modules", "outro", "i.js"), 4096)

	ps := discover(t, root)
	count := 0
	for _, p := range ps {
		count += len(p.Artifacts)
	}
	if count != 1 {
		t.Fatalf("esperado 1 artefato, obtido %d (node_modules aninhado contado duas vezes)", count)
	}
}

// Artifacts are grouped by repository, not by the folder that happened to
// contain them, so a monorepo reads as one project.
func TestGroupsByRepository(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "monorepo")
	mkfile(t, filepath.Join(repo, ".git", "index"), 16)
	for _, pkg := range []string{"web", "api"} {
		mkfile(t, filepath.Join(repo, "apps", pkg, "package.json"), 32)
		mkfile(t, filepath.Join(repo, "apps", pkg, "node_modules", "d", "i.js"), 4096)
	}

	ps := discover(t, root)
	if len(ps) != 1 {
		t.Fatalf("o monorepo deveria ser um projeto so, obtido %d", len(ps))
	}
	if len(ps[0].Artifacts) != 2 {
		t.Fatalf("esperado 2 artefatos no projeto, obtido %d", len(ps[0].Artifacts))
	}
	if ps[0].Name != "monorepo" {
		t.Errorf("projeto deveria se chamar monorepo, chamou %q", ps[0].Name)
	}
}
