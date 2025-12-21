package ui

import (
	"path/filepath"

	"github.com/duxet/dcum/internal/compose"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Root represents the UI application.
type Root struct {
	app   *tview.Application
	table *tview.Table
}

// NewRoot creates a new UI application.
func NewRoot() *Root {
	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false). // Select rows, not columns
		SetFixed(1, 1)              // Fix header row

	return &Root{
		app:   app,
		table: table,
	}
}

// Render displays the list of container images in the table.
func (r *Root) Render(images []compose.ContainerImage) error {
	// Set table headers
	headers := []string{"Service", "Container", "Image", "Current v", "New v", "File"}
	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter).
			SetExpansion(1)

		// "File" column might need more space or less priority, but expansion 1 is fine for now
		if header == "File" {
			cell.SetExpansion(2)
		}

		r.table.SetCell(0, i, cell)
	}

	// Populate rows
	for i, img := range images {
		row := i + 1
		r.table.SetCell(row, 0, tview.NewTableCell(img.ServiceName).SetTextColor(tcell.ColorWhite))
		r.table.SetCell(row, 1, tview.NewTableCell(img.ContainerName).SetTextColor(tcell.ColorWhite))
		r.table.SetCell(row, 2, tview.NewTableCell(img.ImageName).SetTextColor(tcell.ColorGreen))
		r.table.SetCell(row, 3, tview.NewTableCell(img.CurrentVersion).SetTextColor(tcell.ColorBlue).SetAlign(tview.AlignCenter))

		newVerText := img.NewVersion
		newVerColor := tcell.ColorGray
		if newVerText != "" {
			newVerColor = tcell.ColorGreen
		} else {
			newVerText = "-"
		}

		r.table.SetCell(row, 4, tview.NewTableCell(newVerText).SetTextColor(newVerColor).SetAlign(tview.AlignCenter))

		// Show relative path for file if possible
		relPath := img.FilePath
		if wd, err := filepath.Abs("."); err == nil {
			if rel, err := filepath.Rel(wd, img.FilePath); err == nil {
				relPath = rel
			}
		}
		r.table.SetCell(row, 5, tview.NewTableCell(relPath).SetTextColor(tcell.ColorBlue))
	}

	r.app.SetRoot(r.table, true).EnableMouse(true)

	// Add 'q' to quit
	r.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			r.app.Stop()
			return nil
		}
		return event
	})

	return r.app.Run()
}
