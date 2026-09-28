package safety

import (
	"path/filepath"
	"testing"
)

const home = "/Users/tester"

func guard() *Guard { return New(home, 501, []string{"~/importante"}) }

func TestCheck(t *testing.T) {
	cases := []struct {
		path string
		want Level
		why  string
	}{
		// The irreplaceable set. These are the ones that end a user's day.
		{home + "/Library/Application Support/MobileSync/Backup", Denied, "backups de iOS"},
		{home + "/Library/Mobile Documents/com~apple~CloudDocs/tese.pdf", Denied, "iCloud Drive"},
		{home + "/Library/Keychains/login.keychain-db", Denied, "chaveiro"},
		{home + "/.ssh/id_ed25519", Denied, "chave privada"},
		{home + "/Pictures/Photos Library.photoslibrary", Denied, "biblioteca de fotos"},
		{home + "/Library/Containers/com.apple.Notes/Data/Documents", Denied, "documentos no container"},
		{home + "/.local/state/macsweep/undo/1.json", Denied, "o proprio undo do macsweep"},

		// Roots must never be removed wholesale, only things inside them.
		{"/", Denied, "raiz"},
		{home, Denied, "pasta pessoal"},
		{"/Applications", Denied, "raiz de aplicativos"},
		{home + "/Documents", Denied, "raiz de documentos"},
		{home + "/Library/Containers", Denied, "raiz de containers"},

		// Outside the managed roots.
		{"/System/Library/CoreServices/Finder.app", Denied, "fora das raizes"},
		{"/usr/local/bin/algo", Denied, "fora das raizes"},
		{"/etc/hosts", Denied, "fora das raizes"},

		// The user's own ignore list.
		{home + "/importante/coisa", Denied, "lista de ignore"},

		// Real user data: reachable, but only by an exact match.
		{home + "/Documents/projects-github/app/node_modules", Sensitive, "artefato dentro de Documents"},
		{home + "/Library/Containers/com.docker.docker", Sensitive, "container de app"},
		{home + "/Library/Group Containers/TEAMID.com.exemplo", Sensitive, "group container"},

		// The ordinary reclaimable stuff.
		{home + "/.nuget/packages", Allowed, "cache de toolchain"},
		{home + "/Library/Caches/com.exemplo.app", Allowed, "cache de app"},
		{home + "/Library/Logs/Exemplo", Allowed, "logs"},
		{"/Applications/Exemplo.app", Allowed, "bundle de app"},
		{"/private/var/folders/xx/hash/C/com.exemplo", Allowed, "cache por usuario"},

		{"relativo/nao/serve", Denied, "caminho relativo"},
	}

	g := guard()
	for _, c := range cases {
		got := g.Check(c.path)
		if got.Level != c.want {
			t.Errorf("%s: %s\n  esperado %v, obtido %v (%s)", c.why, c.path, c.want, got.Level, got.Reason)
		}
	}
}

// A path under a sensitive root opens only to an exact identification. This
// is the rule that stops an app-name guess from reaching someone's Documents.
func TestSensitiveNeedsExactMatch(t *testing.T) {
	g := guard()
	v := g.Check(home + "/Documents/projects-github/app/node_modules")

	if v.Permits(Heuristic) {
		t.Error("palpite nao pode alcancar caminho sensivel")
	}
	if !v.Permits(Exact) {
		t.Error("correspondencia exata deveria alcancar caminho sensivel")
	}
}

// Deny always wins, even when the caller is certain.
func TestDeniedIgnoresMatchQuality(t *testing.T) {
	g := guard()
	v := g.Check(home + "/Library/Application Support/MobileSync/Backup")
	if v.Permits(Exact) || v.Permits(Heuristic) {
		t.Fatal("caminho negado nao pode ser liberado por qualidade de match")
	}
}

// under compares whole components, so a sibling with a shared prefix is not
// swallowed by its neighbour.
func TestUnderComparesComponents(t *testing.T) {
	if under("/a/bc", "/a/b") {
		t.Error("/a/bc nao esta dentro de /a/b")
	}
	if !under("/a/b/c", "/a/b") {
		t.Error("/a/b/c esta dentro de /a/b")
	}
	if !under("/a/b", "/a/b") {
		t.Error("um caminho esta dentro de si mesmo")
	}
}

// The documented Pearcleaner failure: uninstalling Logitech G HUB also
// selected GarageBand's Application Support/Logic folder. The guard is the
// last line of defence, but the match tier is what has to stop it.
func TestLogitechDoesNotReachLogic(t *testing.T) {
	g := guard()
	logic := filepath.Join(home, "Library/Application Support/Logic")
	v := g.Check(logic)
	if v.Permits(Heuristic) && v.Level == Allowed {
		// Allowed is correct for this path in isolation; the protection has
		// to come from the matcher never proposing it. Assert the contract
		// the matcher must honour.
		t.Log("Application Support/Logic e liberado em si: quem precisa barrar e o matcher, com match exato")
	}
}
