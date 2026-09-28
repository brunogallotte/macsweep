// Package ui holds the shared look of macsweep: one palette, one set of key
// bindings, one way of writing a size. Every module renders through here, so
// the tool reads as one thing rather than four.
package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

// Theme carries precomputed styles. Building a lipgloss.Style is not free, so
// they are made once here and never inside a View.
type Theme struct {
	Title    lipgloss.Style
	Subtle   lipgloss.Style
	Accent   lipgloss.Style
	Size     lipgloss.Style
	Warn     lipgloss.Style
	Danger   lipgloss.Style
	Good     lipgloss.Style
	Cursor   lipgloss.Style
	Selected lipgloss.Style
	Locked   lipgloss.Style
	Header   lipgloss.Style
	Footer   lipgloss.Style
	Key      lipgloss.Style
	BarFill  lipgloss.Style
	BarTrack lipgloss.Style
}

// NewTheme builds the palette. dark says whether the terminal background is
// dark; when unknown, dark is the safer assumption because most terminals are.
func NewTheme(dark bool) *Theme {
	ld := lipgloss.LightDark(dark)

	var (
		fg     = ld(lipgloss.Color("#1f2430"), lipgloss.Color("#e6e9ef"))
		muted  = ld(lipgloss.Color("#6b7280"), lipgloss.Color("#8b93a7"))
		accent = ld(lipgloss.Color("#0d9488"), lipgloss.Color("#2dd4bf"))
		warn   = ld(lipgloss.Color("#b45309"), lipgloss.Color("#fbbf24"))
		danger = ld(lipgloss.Color("#b91c1c"), lipgloss.Color("#f87171"))
		good   = ld(lipgloss.Color("#15803d"), lipgloss.Color("#4ade80"))
		track  = ld(lipgloss.Color("#d1d5db"), lipgloss.Color("#374151"))
	)

	return &Theme{
		Title:    lipgloss.NewStyle().Foreground(fg).Bold(true),
		Subtle:   lipgloss.NewStyle().Foreground(muted),
		Accent:   lipgloss.NewStyle().Foreground(accent),
		Size:     lipgloss.NewStyle().Foreground(fg).Bold(true),
		Warn:     lipgloss.NewStyle().Foreground(warn),
		Danger:   lipgloss.NewStyle().Foreground(danger),
		Good:     lipgloss.NewStyle().Foreground(good),
		Cursor:   lipgloss.NewStyle().Foreground(accent).Bold(true),
		Selected: lipgloss.NewStyle().Foreground(accent),
		Locked:   lipgloss.NewStyle().Foreground(muted).Strikethrough(true),
		Header:   lipgloss.NewStyle().Foreground(muted).Bold(true),
		Footer:   lipgloss.NewStyle().Foreground(fg),
		Key:      lipgloss.NewStyle().Foreground(accent).Bold(true),
		BarFill:  lipgloss.NewStyle().Foreground(accent),
		BarTrack: lipgloss.NewStyle().Foreground(track),
	}
}

// NewPlainTheme drops every colour while keeping the layout identical, for
// pipes, NO_COLOR and anything that is not a terminal.
func NewPlainTheme() *Theme {
	n := lipgloss.NewStyle()
	return &Theme{
		Title: n, Subtle: n, Accent: n, Size: n, Warn: n, Danger: n, Good: n,
		Cursor: n, Selected: n, Locked: n, Header: n, Footer: n, Key: n,
		BarFill: n, BarTrack: n,
	}
}

// Plain reports whether colour should be suppressed: NO_COLOR is set, or
// output is not going to a terminal.
func Plain() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return true
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// Size renders a byte count the way a person reads it.
func Size(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	v := float64(b) / float64(div)
	if v >= 100 {
		return fmt.Sprintf("%.0f %cB", v, "KMGTPE"[exp])
	}
	return fmt.Sprintf("%.1f %cB", v, "KMGTPE"[exp])
}

// Bar draws a proportional bar, used everywhere a size is shown next to its
// share of a total.
func (t *Theme) SizeBar(value, total int64, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat(" ", max(width, 0))
	}
	filled := int(float64(value) / float64(total) * float64(width))
	filled = min(max(filled, 0), width)
	// A non-zero value always shows at least a sliver, otherwise small items
	// look like nothing at all.
	if filled == 0 && value > 0 {
		filled = 1
	}
	return t.BarFill.Render(strings.Repeat("█", filled)) +
		t.BarTrack.Render(strings.Repeat("░", width-filled))
}

// Keys renders a hint line like "espaco marcar   enter limpar".
func (t *Theme) Keys(pairs ...string) string {
	var b strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			b.WriteString("   ")
		}
		b.WriteString(t.Key.Render(pairs[i]))
		b.WriteString(" ")
		b.WriteString(t.Subtle.Render(pairs[i+1]))
	}
	return b.String()
}

// Truncate shortens a string to width, keeping the tail of a path visible
// because the end of a path carries more information than its start.
func TruncatePath(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return "..." + s[len(s)-(width-3):]
}
