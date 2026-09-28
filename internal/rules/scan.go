package rules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/brunogallotte/macsweep/internal/scan"
)

// Finding is one concrete thing a rule matched.
type Finding struct {
	Rule Rule   `json:"rule"`
	Path string `json:"path,omitempty"`
	Size int64  `json:"size"`
}

// Scan resolves every rule at or below the given risk level and measures what
// it found. Rules are evaluated in parallel because most of the cost is
// walking directories.
func Scan(ctx context.Context, home string, maxRisk Risk) []Finding {
	var (
		mu  sync.Mutex
		out []Finding
		wg  sync.WaitGroup
		sem = make(chan struct{}, 8)
	)

	for _, r := range Catalog {
		if !allows(maxRisk, r.Risk) {
			continue
		}
		wg.Add(1)
		go func(r Rule) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			found := evaluate(ctx, home, r)
			if len(found) == 0 {
				return
			}
			mu.Lock()
			out = append(out, found...)
			mu.Unlock()
		}(r)
	}
	wg.Wait()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule.Tool != out[j].Rule.Tool {
			return out[i].Rule.Tool < out[j].Rule.Tool
		}
		return out[i].Size > out[j].Size
	})
	return out
}

func allows(maxRisk, r Risk) bool {
	order := map[Risk]int{Safe: 0, Moderate: 1, Risky: 2}
	return order[r] <= order[maxRisk]
}

func evaluate(ctx context.Context, home string, r Rule) []Finding {
	if len(r.Command) > 0 {
		if _, err := exec.LookPath(r.Requires); err != nil {
			return nil
		}
		size := commandSize(r)
		if size == 0 {
			return nil
		}
		return []Finding{{Rule: r, Size: size}}
	}

	var out []Finding
	for _, pattern := range r.Globs {
		matches, err := filepath.Glob(expand(pattern, home))
		if err != nil {
			continue
		}
		for _, m := range matches {
			res, err := scan.Walk(ctx, m, scan.Options{})
			if err != nil || res.Root.Size == 0 {
				continue
			}
			out = append(out, Finding{Rule: r, Path: m, Size: res.Root.Size})
		}
	}
	return out
}

func expand(pattern, home string) string {
	if strings.HasPrefix(pattern, "~/") {
		pattern = filepath.Join(home, pattern[2:])
	}
	if strings.Contains(pattern, "$DARWIN_USER_CACHE_DIR") {
		dir := darwinUserCacheDir()
		if dir == "" {
			return ""
		}
		pattern = strings.ReplaceAll(pattern, "$DARWIN_USER_CACHE_DIR", strings.TrimSuffix(dir, "/"))
	}
	return pattern
}

var cacheDirOnce = sync.OnceValue(func() string {
	out, err := exec.Command("getconf", "DARWIN_USER_CACHE_DIR").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
})

func darwinUserCacheDir() string { return cacheDirOnce() }

// commandSize asks the owning tool how much it is holding, so a command rule
// can be listed with a real number instead of a shrug.
func commandSize(r Rule) int64 {
	switch r.ID {
	case "docker":
		return dockerReclaimable("Images", "Local Volumes", "Containers")
	case "docker-builder":
		return dockerReclaimable("Build Cache")
	case "go-modcache":
		return dirSize(goEnv("GOMODCACHE"))
	case "go-buildcache":
		return dirSize(goEnv("GOCACHE"))
	default:
		// Unknown but present. Reporting one byte keeps it visible in the
		// list without inventing a number.
		return 1
	}
}

// dockerReclaimable reads `docker system df` and sums the reclaimable column
// for the requested types.
func dockerReclaimable(types ...string) int64 {
	out, err := exec.Command("docker", "system", "df").Output()
	if err != nil {
		return 0
	}
	want := map[string]bool{}
	for _, t := range types {
		want[t] = true
	}

	var total int64
	for _, line := range strings.Split(string(out), "\n")[1:] {
		fields := strings.Split(line, "  ")
		var cols []string
		for _, f := range fields {
			if f = strings.TrimSpace(f); f != "" {
				cols = append(cols, f)
			}
		}
		if len(cols) < 5 || !want[cols[0]] {
			continue
		}
		total += parseDockerSize(cols[len(cols)-1])
	}
	return total
}

// parseDockerSize reads values like "1.234GB (85%)".
func parseDockerSize(s string) int64 {
	if i := strings.Index(s, "("); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	units := []struct {
		suffix string
		mult   float64
	}{
		{"TB", 1 << 40}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"kB", 1 << 10}, {"B", 1},
	}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.suffix); ok {
			n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return 0
			}
			return int64(n * u.mult)
		}
	}
	return 0
}

func goEnv(key string) string {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func dirSize(path string) int64 {
	if path == "" {
		return 0
	}
	if _, err := os.Stat(path); err != nil {
		return 0
	}
	res, err := scan.Walk(context.Background(), path, scan.Options{})
	if err != nil {
		return 0
	}
	return res.Root.Size
}
