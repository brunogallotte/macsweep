// Package rules is the declarative catalogue of what macsweep knows how to
// clean. Everything lives in one table on purpose: adding coverage should be
// a data change, and auditing what the tool touches should be reading a list
// rather than reading code.
package rules

// Risk decides what comes preselected and what has to be asked for.
type Risk string

const (
	// Safe is pure cache: removing it costs a re-download at worst.
	Safe Risk = "seguro"
	// Moderate costs real time to rebuild, or changes how a tool behaves.
	Moderate Risk = "moderado"
	// Risky can lose something the user wanted. Hidden behind --risky.
	Risky Risk = "arriscado"
)

// Rule is one cleanable thing.
type Rule struct {
	ID    string
	Title string
	Tool  string // the group it is listed under
	Risk  Risk

	// Globs are shell style patterns, with ~ meaning the home directory.
	// Every match is checked by the safety guard before it is offered.
	Globs []string

	// Command replaces path deletion for things only their own tool can
	// clean up correctly, such as a Docker prune.
	Command []string

	// Requires is a binary that must be on PATH for a Command rule.
	Requires string

	// Regenerate is the one line explanation of how it comes back. It is the
	// difference between a user confidently ticking a box and closing the
	// tool.
	Regenerate string
}

// Catalog is every rule macsweep ships.
var Catalog = []Rule{
	// Package manager caches. On a developer machine this is usually the
	// single largest pile of recoverable bytes outside of build output.
	{ID: "nuget", Title: "Pacotes NuGet", Tool: "dotnet", Risk: Safe,
		Globs: []string{"~/.nuget/packages"}, Regenerate: "dotnet restore baixa de novo"},
	{ID: "dotnet-cache", Title: "Cache do SDK .NET", Tool: "dotnet", Risk: Safe,
		Globs: []string{"~/.dotnet/optimizationdata", "~/Library/Caches/NuGet"}, Regenerate: "recriado no proximo build"},

	{ID: "npm", Title: "Cache do npm", Tool: "node", Risk: Safe,
		Globs: []string{"~/.npm/_cacache"}, Regenerate: "npm baixa de novo no proximo install"},
	{ID: "pnpm", Title: "Store do pnpm", Tool: "node", Risk: Moderate,
		Globs: []string{"~/Library/pnpm/store", "~/.pnpm-store"}, Regenerate: "pnpm install rebaixa tudo"},
	{ID: "yarn", Title: "Cache do Yarn", Tool: "node", Risk: Safe,
		Globs: []string{"~/Library/Caches/Yarn", "~/.yarn/cache"}, Regenerate: "yarn install baixa de novo"},
	{ID: "bun", Title: "Cache do Bun", Tool: "node", Risk: Safe,
		Globs: []string{"~/.bun/install/cache"}, Regenerate: "bun install baixa de novo"},

	{ID: "cargo", Title: "Registro do Cargo", Tool: "rust", Risk: Safe,
		Globs: []string{"~/.cargo/registry/cache", "~/.cargo/registry/src"}, Regenerate: "cargo build baixa de novo"},
	{ID: "rustup", Title: "Toolchains do rustup", Tool: "rust", Risk: Moderate,
		Globs: []string{"~/.rustup/downloads", "~/.rustup/tmp"}, Regenerate: "rustup baixa quando precisar"},

	{ID: "maven", Title: "Repositorio Maven", Tool: "java", Risk: Moderate,
		Globs: []string{"~/.m2/repository"}, Regenerate: "mvn baixa de novo, leva tempo"},
	{ID: "gradle", Title: "Caches do Gradle", Tool: "java", Risk: Moderate,
		Globs: []string{"~/.gradle/caches", "~/.gradle/daemon"}, Regenerate: "gradle reconstroi no proximo build"},

	{ID: "pip", Title: "Cache do pip", Tool: "python", Risk: Safe,
		Globs: []string{"~/Library/Caches/pip"}, Regenerate: "pip baixa de novo"},
	{ID: "uv", Title: "Cache do uv", Tool: "python", Risk: Safe,
		Globs: []string{"~/Library/Caches/uv", "~/.cache/uv"}, Regenerate: "uv baixa de novo"},
	{ID: "poetry", Title: "Cache do Poetry", Tool: "python", Risk: Safe,
		Globs: []string{"~/Library/Caches/pypoetry"}, Regenerate: "poetry baixa de novo"},

	{ID: "asdf", Title: "Downloads do asdf", Tool: "versoes", Risk: Safe,
		Globs: []string{"~/.asdf/downloads"}, Regenerate: "apenas instaladores ja usados"},
	{ID: "mise", Title: "Cache do mise", Tool: "versoes", Risk: Safe,
		Globs: []string{"~/.cache/mise", "~/Library/Caches/mise"}, Regenerate: "mise baixa de novo"},
	{ID: "homebrew", Title: "Downloads do Homebrew", Tool: "versoes", Risk: Safe,
		Globs: []string{"~/Library/Caches/Homebrew"}, Regenerate: "brew baixa de novo"},

	{ID: "cocoapods", Title: "Cache do CocoaPods", Tool: "swift", Risk: Safe,
		Globs: []string{"~/Library/Caches/CocoaPods"}, Regenerate: "pod install baixa de novo"},
	{ID: "xcode-derived", Title: "DerivedData do Xcode", Tool: "swift", Risk: Safe,
		Globs: []string{"~/Library/Developer/Xcode/DerivedData/*"}, Regenerate: "recriado no proximo build"},
	{ID: "xcode-devicesupport", Title: "Symbols de dispositivos iOS", Tool: "swift", Risk: Moderate,
		Globs: []string{"~/Library/Developer/Xcode/iOS DeviceSupport/*"}, Regenerate: "recopiado ao conectar o aparelho"},
	{ID: "xcode-archives", Title: "Archives do Xcode", Tool: "swift", Risk: Risky,
		Globs: []string{"~/Library/Developer/Xcode/Archives/*"}, Regenerate: "nao volta, sao builds assinados"},
	{ID: "simulator-caches", Title: "Caches do Simulador", Tool: "swift", Risk: Safe,
		Globs: []string{"~/Library/Developer/CoreSimulator/Caches/*"}, Regenerate: "recriado pelo simulador"},

	// Electron and Chromium apps keep several parallel caches each. These
	// patterns cover the usual suspects without naming every app.
	{ID: "electron-cache", Title: "Cache de apps Electron", Tool: "aplicativos", Risk: Safe,
		Globs: []string{
			"~/Library/Application Support/*/Cache",
			"~/Library/Application Support/*/Code Cache",
			"~/Library/Application Support/*/GPUCache",
			"~/Library/Application Support/*/DawnCache",
			"~/Library/Application Support/*/ShaderCache",
			"~/Library/Application Support/*/blob_storage",
			"~/Library/Application Support/*/Service Worker/CacheStorage",
			"~/Library/Application Support/*/*/Cache",
			"~/Library/Application Support/*/*/Code Cache",
			"~/Library/Application Support/*/*/GPUCache",
			"~/Library/Application Support/*/*/Service Worker/CacheStorage",
		},
		Regenerate: "os apps reconstroem sozinhos ao abrir"},
	{ID: "editor-cacheddata", Title: "Cache de versoes antigas do editor", Tool: "aplicativos", Risk: Safe,
		Globs: []string{
			"~/Library/Application Support/Code/CachedData",
			"~/Library/Application Support/Cursor/CachedData",
			"~/Library/Application Support/Code/CachedExtensionVSIXs",
			"~/Library/Application Support/Cursor/CachedExtensionVSIXs",
		},
		Regenerate: "recriado ao abrir o editor"},
	{ID: "crashpad", Title: "Relatorios de crash de apps", Tool: "aplicativos", Risk: Safe,
		Globs:      []string{"~/Library/Application Support/*/Crashpad", "~/Library/Application Support/*/*/Crashpad"},
		Regenerate: "nao precisa voltar"},

	// System level caches and logs.
	{ID: "user-caches", Title: "Caches do usuario", Tool: "sistema", Risk: Moderate,
		Globs: []string{"~/Library/Caches/*"}, Regenerate: "cada app recria o seu"},
	{ID: "user-logs", Title: "Logs de aplicativos", Tool: "sistema", Risk: Safe,
		Globs: []string{"~/Library/Logs/*"}, Regenerate: "nao precisa voltar"},
	{ID: "saved-state", Title: "Estado de janelas salvo", Tool: "sistema", Risk: Moderate,
		Globs: []string{"~/Library/Saved Application State/*"}, Regenerate: "os apps abrem sem restaurar janelas uma vez"},
	{ID: "temp-cache", Title: "Cache temporario por usuario", Tool: "sistema", Risk: Safe,
		Globs: []string{"$DARWIN_USER_CACHE_DIR/*"}, Regenerate: "recriado automaticamente"},
	{ID: "quicklook", Title: "Cache de pre-visualizacao", Tool: "sistema", Risk: Safe,
		Globs: []string{"$DARWIN_USER_CACHE_DIR/com.apple.QuickLook.thumbnailcache"}, Regenerate: "recriado ao abrir o Finder"},

	// Things only their own tool can clean correctly.
	{ID: "docker", Title: "Imagens e cache do Docker", Tool: "docker", Risk: Moderate,
		Command: []string{"docker", "system", "prune", "-f"}, Requires: "docker",
		Regenerate: "as imagens sao baixadas ou reconstruidas de novo"},
	{ID: "docker-builder", Title: "Cache de build do Docker", Tool: "docker", Risk: Safe,
		Command: []string{"docker", "builder", "prune", "-f"}, Requires: "docker",
		Regenerate: "o proximo build reconstroi as camadas"},
	{ID: "go-modcache", Title: "Cache de modulos Go", Tool: "go", Risk: Safe,
		Command: []string{"go", "clean", "-modcache"}, Requires: "go",
		Regenerate: "go baixa de novo no proximo build"},
	{ID: "go-buildcache", Title: "Cache de build do Go", Tool: "go", Risk: Safe,
		Command: []string{"go", "clean", "-cache"}, Requires: "go",
		Regenerate: "recriado no proximo build"},
	{ID: "brew-cleanup", Title: "Versoes antigas do Homebrew", Tool: "versoes", Risk: Safe,
		Command: []string{"brew", "cleanup", "--prune=all"}, Requires: "brew",
		Regenerate: "remove apenas versoes ja substituidas"},
	{ID: "simctl-unavailable", Title: "Simuladores indisponiveis", Tool: "swift", Risk: Safe,
		Command: []string{"xcrun", "simctl", "delete", "unavailable"}, Requires: "xcrun",
		Regenerate: "so remove simuladores que nao rodam mais"},
}
