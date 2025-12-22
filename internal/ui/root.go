package ui

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/duxet/dcum/internal/compose"
	"github.com/duxet/dcum/internal/registry"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Root represents the UI application.
type Root struct {
	app       *tview.Application
	table     *tview.Table
	statusBar *tview.TextView
	images    []compose.ContainerImage
	checking  map[int]bool // Track which rows are currently being checked
}

// NewRoot creates a new UI application.
func NewRoot() *Root {
	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false). // Select rows, not columns
		SetFixed(1, 1)              // Fix header row

	statusBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("Loading...")

	return &Root{
		app:       app,
		table:     table,
		statusBar: statusBar,
	}
}

// Render displays the list of container images in the table.
func (r *Root) Render(images []compose.ContainerImage, checker *registry.Checker) error {
	r.images = images
	r.checking = make(map[int]bool)

	// Create layout
	grid := tview.NewGrid().
		SetRows(0, 1).
		SetColumns(0).
		SetBorders(false)

	grid.AddItem(r.table, 0, 0, 1, 1, 0, 0, true)
	grid.AddItem(r.statusBar, 1, 0, 1, 1, 0, 0, false)

	r.refreshTable()
	r.updateStatusBar()

	r.table.SetSelectedFunc(func(row, column int) {
		if row > 0 && row <= len(r.images) {
			r.cycleVersion(row - 1)
			r.refreshTable()
		}
	})

	r.app.SetRoot(grid, true).EnableMouse(true)

	// Add 'q' to quit, 's' to save
	r.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			r.app.Stop()
			return nil
		}
		if event.Rune() == 's' {
			r.saveChanges()
			return nil
		}
		return event
	})

	// Start async check
	go r.checkUpdates(checker)

	return r.app.Run()
}

func (r *Root) checkUpdates(checker *registry.Checker) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Limit concurrency to 5

	for i := range r.images {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Mark as checking
			r.app.QueueUpdateDraw(func() {
				r.checking[idx] = true
				r.refreshTable()
			})

			tagRegex := ""
			if r.images[idx].Labels != nil {
				if val, ok := r.images[idx].Labels["wud.tag.include"]; ok {
					tagRegex = val
				}
			}

			candidates, err := checker.GetUpdateCandidates(r.images[idx].ImageName, r.images[idx].CurrentVersion, tagRegex)

			// Update image data in main thread safe way
			r.app.QueueUpdateDraw(func() {
				r.checking[idx] = false
				if err != nil {
					// We could store error state to show in UI
				} else {
					r.images[idx].UpdatePatch = candidates.Patch
					r.images[idx].UpdateMinor = candidates.Minor
					r.images[idx].UpdateMajor = candidates.Major

					// Default selection priority: Patch > Minor > Major
					if candidates.Patch != "" {
						r.images[idx].NewVersion = candidates.Patch
					} else if candidates.Minor != "" {
						r.images[idx].NewVersion = candidates.Minor
					} else if candidates.Major != "" {
						r.images[idx].NewVersion = candidates.Major
					}
				}
				r.refreshTable()
			})
		}(i)
	}
	wg.Wait()
}

func (r *Root) updateStatusBar() {
	r.statusBar.SetText(" [bold]q[::-] Quit | [bold]s[::-] Save Changes | [bold]Enter[::-] Cycle Version | [bold]Up/Down[::-] Navigate")
}

func (r *Root) saveChanges() {
	scanner := compose.NewScanner()
	if err := scanner.UpdateImages(r.images); err != nil {
		// Show error modal (simplification: just print to stderr for now or panic?)
		// To show in TUI needs a valid modal primitive.
		// Let's print to stdout/stderr which might mess selection but at least logic runs.
		// Better: Change title of table to show error?
		r.table.SetTitle(fmt.Sprintf("Error saving: %v", err))
		return
	}
	r.table.SetTitle("Changes saved successfully!")

	// Reload/Reset UI state? modify current versions in memory?
	for i, img := range r.images {
		if img.NewVersion != "" {
			r.images[i].CurrentVersion = img.NewVersion
			r.images[i].NewVersion = ""
			r.images[i].UpdatePatch = "" // Clear suggestions as they are invalid now
			r.images[i].UpdateMinor = ""
			r.images[i].UpdateMajor = ""
			// In reality, we should re-scan to get fresh state, but simple update works.
		}
	}
	r.refreshTable()
}

func (r *Root) cycleVersion(index int) {
	img := &r.images[index]

	// Cycle: Current (empty NewVersion) -> Patch -> Minor -> Major -> Current...
	// If any candidate is missing, skip it in the cycle.

	switch img.NewVersion {
	case "":
		// Currently no update selected. Try selecting Patch, then Minor, then Major.
		if img.UpdatePatch != "" {
			img.NewVersion = img.UpdatePatch
		} else if img.UpdateMinor != "" {
			img.NewVersion = img.UpdateMinor
		} else if img.UpdateMajor != "" {
			img.NewVersion = img.UpdateMajor
		}
	case img.UpdatePatch:
		// Currently Patch selected. Try Minor, then Major, then None.
		if img.UpdateMinor != "" {
			img.NewVersion = img.UpdateMinor
		} else if img.UpdateMajor != "" {
			img.NewVersion = img.UpdateMajor
		} else {
			img.NewVersion = "" // Reset
		}
	case img.UpdateMinor:
		// Currently Minor selected. Try Major, then None.
		if img.UpdateMajor != "" {
			img.NewVersion = img.UpdateMajor
		} else {
			img.NewVersion = "" // Reset
		}
	case img.UpdateMajor:
		// Currently Major selected. Go to None.
		img.NewVersion = ""
	default:
		// Unknown state, reset
		img.NewVersion = ""
	}
}

func (r *Root) refreshTable() {
	r.table.Clear()

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
	for i, img := range r.images {
		row := i + 1
		r.table.SetCell(row, 0, tview.NewTableCell(img.ServiceName).SetTextColor(tcell.ColorWhite))
		r.table.SetCell(row, 1, tview.NewTableCell(img.ContainerName).SetTextColor(tcell.ColorWhite))
		r.table.SetCell(row, 2, tview.NewTableCell(img.ImageName).SetTextColor(tcell.ColorGreen))
		r.table.SetCell(row, 3, tview.NewTableCell(img.CurrentVersion).SetTextColor(tcell.ColorBlue).SetAlign(tview.AlignCenter))

		newVerText := img.NewVersion
		newVerColor := tcell.ColorGray

		// Add indicators
		if newVerText == "" {
			newVerText = "-"
		} else {
			if newVerText == img.UpdateMajor {
				newVerText += " (Maj)"
				newVerColor = tcell.ColorRed
			} else if newVerText == img.UpdateMinor {
				newVerText += " (Min)"
				newVerColor = tcell.ColorYellow
			} else if newVerText == img.UpdatePatch {
				newVerText += " (Pat)"
				newVerColor = tcell.ColorGreen
			} else {
				// Should not happen if logic is correct, but fallback
				newVerColor = tcell.ColorWhite
			}
		}

		if r.checking[i] {
			newVerText = "checking..."
			newVerColor = tcell.ColorYellow
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
}
