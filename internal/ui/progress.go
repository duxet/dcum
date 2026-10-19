package ui

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"
)

// FormatProgressBar returns a formatted string for a progress bar with dynamic tview color tags.
func FormatProgressBar(current, total, width int) string {
	if width <= 0 {
		width = 30
	}
	if total <= 0 {
		total = 1
		current = 1
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}

	percent := float64(current) / float64(total) * 100
	filledLen := int(float64(width) * float64(current) / float64(total))
	if filledLen > width {
		filledLen = width
	}
	emptyLen := width - filledLen

	filled := strings.Repeat("█", filledLen)
	empty := strings.Repeat("░", emptyLen)

	return fmt.Sprintf("Fetching latest versions: %s[green]%s[darkgray]%s[-]] %3.0f%% (%d/%d)",
		tview.Escape("["),
		filled,
		empty,
		percent,
		current,
		total,
	)
}
