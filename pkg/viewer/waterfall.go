package viewer

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Colors and styles
var (
	spanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	errorSpanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF"))

	durationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	barFilledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4"))

	barEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444"))
)

// WaterfallRenderer renders spans as a waterfall chart
type WaterfallRenderer struct {
	Width      int
	ShowBars   bool
	ShowTiming bool
}

// NewWaterfallRenderer creates a new waterfall renderer
func NewWaterfallRenderer(width int) *WaterfallRenderer {
	return &WaterfallRenderer{
		Width:      width,
		ShowBars:   true,
		ShowTiming: true,
	}
}

// RenderNode renders a single trace node
func (w *WaterfallRenderer) RenderNode(node *TraceNode, maxDuration time.Duration) string {
	var sb strings.Builder

	// Indentation based on depth
	indent := strings.Repeat("  ", node.Depth)
	if node.Depth > 0 {
		indent = indent[:len(indent)-2] + "└─"
	}

	// Span name with icon
	icon := "▸"
	if node.Expanded && len(node.Children) > 0 {
		icon = "▾"
	} else if len(node.Children) == 0 {
		icon = "•"
	}

	spanName := fmt.Sprintf("%s %s %s", indent, icon, node.Span.Name)

	// Apply styling
	style := spanStyle
	if node.Span.StatusCode != 0 {
		style = errorSpanStyle
	}
	if node.Selected {
		style = selectedStyle
	}

	sb.WriteString(style.Render(spanName))

	// Duration
	duration := node.Span.EndTime.Sub(node.Span.StartTime)
	durationStr := fmt.Sprintf("(%s)", formatDuration(duration))
	sb.WriteString("  ")
	sb.WriteString(durationStyle.Render(durationStr))

	// Bar chart
	if w.ShowBars && maxDuration > 0 {
		sb.WriteString("  ")
		sb.WriteString(w.renderBar(duration, maxDuration, 20))
	}

	return sb.String()
}

// renderBar renders a horizontal bar chart
func (w *WaterfallRenderer) renderBar(duration, maxDuration time.Duration, width int) string {
	if maxDuration == 0 {
		return ""
	}

	ratio := float64(duration) / float64(maxDuration)
	filled := int(ratio * float64(width))

	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	return barFilledStyle.Render(bar[:filled]) + barEmptyStyle.Render(bar[filled:])
}

// RenderTree renders the entire tree
func (w *WaterfallRenderer) RenderTree(tree *TraceTree) []string {
	nodes := tree.FlattenVisible()
	if len(nodes) == 0 {
		return []string{"No traces to display"}
	}

	// Find max duration for scaling
	var maxDuration time.Duration
	for _, node := range nodes {
		duration := node.Span.EndTime.Sub(node.Span.StartTime)
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	lines := make([]string, 0, len(nodes))
	for _, node := range nodes {
		lines = append(lines, w.RenderNode(node, maxDuration))
	}

	return lines
}

// RenderHeader renders the header with timestamp
func (w *WaterfallRenderer) RenderHeader() string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	timestamp := time.Now().Format("15:04:05")
	return style.Render(fmt.Sprintf("Local Trace Tap - %s", timestamp))
}

// RenderStats renders the statistics panel
func (w *WaterfallRenderer) RenderStats(stats TraceStats) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2)

	content := fmt.Sprintf(`Statistics
─────────────────────
Total Spans:    %d
Avg Latency:    %s
Error Rate:     %.2f%%`,
		stats.TotalSpans,
		formatDuration(stats.AvgLatency),
		stats.ErrorRate,
	)

	return style.Render(content)
}

// RenderHelp renders the help text
func (w *WaterfallRenderer) RenderHelp() string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	return style.Render("↑/↓: Navigate | Enter: Expand/Collapse | /: Filter | e: Export | q: Quit")
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
