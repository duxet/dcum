package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/duxet/dcum/internal/compose"
	"github.com/duxet/dcum/internal/config"
	"github.com/duxet/dcum/internal/registry"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// CheckState represents the status of an update check.
type CheckState int

const (
	CheckStatePending CheckState = iota
	CheckStateChecking
	CheckStateDone
)

// Root represents the UI application.
type Root struct {
	app              *tview.Application
	table            *tview.Table
	statusBar        *tview.TextView
	images           []compose.ContainerImage
	checkStatus      map[int]CheckState // Track status of each row
	config           *config.Config
	filterUpdates    bool
	displayedIndices []int // Maps table row to index in images slice
}

// NewRoot creates a new UI application.
func NewRoot(cfg *config.Config) *Root {
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
		app:              app,
		table:            table,
		statusBar:        statusBar,
		config:           cfg,
		displayedIndices: []int{},
	}
}

// Render displays the list of container images in the table.
func (r *Root) Render(images []compose.ContainerImage, checker *registry.Checker) error {
	r.images = images
	r.checkStatus = make(map[int]CheckState)

	// Initialize displayedIndices with all images
	r.displayedIndices = make([]int, len(r.images))
	for i := range r.images {
		r.displayedIndices[i] = i
	}

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
		if row > 0 && row <= len(r.displayedIndices) {
			r.cycleVersion(r.displayedIndices[row-1])
			r.refreshTable()
		}
	})

	r.app.SetRoot(grid, true).EnableMouse(true)

	// Add 'q' to quit, 's' to save, 'r' to refresh
	r.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			r.app.Stop()
			return nil
		}
		if event.Rune() == 's' {
			r.saveChanges()
			return nil
		}
		if event.Rune() == 'r' {
			go r.checkUpdates(checker, true)
			return nil
		}
		if event.Rune() == 'u' {
			r.filterUpdates = !r.filterUpdates
			r.refreshTable()
			r.updateStatusBar()
			return nil
		}
		return event
	})

	// Start async check
	go r.checkUpdates(checker, false)

	return r.app.Run()
}

func (r *Root) checkUpdates(checker *registry.Checker, forceRefresh bool) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Limit concurrency to 5

	// Initialize all as pending first
	r.app.QueueUpdateDraw(func() {
		for i := range r.images {
			if _, ok := r.checkStatus[i]; !ok {
				r.checkStatus[i] = CheckStatePending
			}
		}
		r.refreshTable()
	})

	for i := range r.images {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Mark as checking
			r.app.QueueUpdateDraw(func() {
				r.checkStatus[idx] = CheckStateChecking
				r.refreshTable()
			})

			includeRegex := ""
			excludeRegex := ""
			if r.images[idx].Labels != nil {
				if val, ok := r.images[idx].Labels["wud.tag.include"]; ok {
					includeRegex = val
				}
				if val, ok := r.images[idx].Labels["wud.tag.exclude"]; ok {
					excludeRegex = val
				}
			}

			candidates, err := checker.GetUpdateCandidates(r.images[idx].ImageName, r.images[idx].CurrentVersion, includeRegex, excludeRegex, forceRefresh)

			// Update image data in main thread safe way
			r.app.QueueUpdateDraw(func() {
				r.checkStatus[idx] = CheckStateDone
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
	r.statusBar.SetText(" [bold]q[::-] Quit | [bold]s[::-] Save Changes | [bold]r[::-] Refresh | [bold]u[::-] Toggle Updates Only | [bold]Enter[::-] Cycle Version | [bold]Up/Down[::-] Navigate")
}

func (r *Root) saveChanges() {
	scanner := compose.NewScanner([]string{}) // No exclusion needed for saving
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

	// Define columns based on config
	headers := []string{"File", "Service", "Image", "Current v", "New v"}
	if r.config.UI.ShowContainer {
		headers = append(headers, "Container")
	}

	for i, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetSelectable(false).
			SetAlign(tview.AlignCenter).
			SetExpansion(1)

		if header == "File" {
			cell.SetExpansion(2)
		}

		r.table.SetCell(0, i, cell)
	}

	// Update displayedIndices based on filter
	r.displayedIndices = []int{}
	for i, img := range r.images {
		if r.filterUpdates {
			if img.UpdatePatch == "" && img.UpdateMinor == "" && img.UpdateMajor == "" {
				continue
			}
		}
		r.displayedIndices = append(r.displayedIndices, i)
	}

	// Populate rows
	for i, idx := range r.displayedIndices {
		img := r.images[idx]
		row := i + 1
		col := 0

		// Column 0: File
		relPath := img.FilePath
		if wd, err := filepath.Abs("."); err == nil {
			if rel, err := filepath.Rel(wd, img.FilePath); err == nil {
				relPath = rel
			}
		}
		r.table.SetCell(row, col, tview.NewTableCell(relPath).SetTextColor(tcell.ColorBlue))
		col++

		// Column 1: Service
		r.table.SetCell(row, col, tview.NewTableCell(img.ServiceName).SetTextColor(tcell.ColorWhite))
		col++

		// Column 2: Image
		r.table.SetCell(row, col, tview.NewTableCell(img.ImageName).SetTextColor(tcell.ColorGreen))
		col++

		// Column 3: Current v
		r.table.SetCell(row, col, tview.NewTableCell(truncateVersion(img.CurrentVersion)).SetTextColor(tcell.ColorBlue).SetAlign(tview.AlignCenter))
		col++

		// Column 4: New v
		newVerText := img.NewVersion
		newVerColor := tcell.ColorGray

		// Add indicators
		state, ok := r.checkStatus[idx]
		if !ok || state == CheckStatePending {
			newVerText = "waiting..."
			newVerColor = tcell.ColorGray
		} else if state == CheckStateChecking {
			newVerText = "checking..."
			newVerColor = tcell.ColorYellow
		} else {
			// CheckStateDone
			if newVerText == "" {
				newVerText = "-"
			} else {
				isMaj := newVerText == img.UpdateMajor
				isMin := newVerText == img.UpdateMinor
				isPat := newVerText == img.UpdatePatch

				newVerText = truncateVersion(newVerText)

				if isMaj {
					newVerText += " (Maj)"
					newVerColor = tcell.ColorRed
					if img.UpdateMinor != "" || img.UpdatePatch != "" {
						newVerText += " ↓"
					}
				} else if isMin {
					newVerText += " (Min)"
					newVerColor = tcell.ColorYellow
					if img.UpdateMajor != "" {
						newVerText += " ↑"
					}
					if img.UpdatePatch != "" {
						newVerText += " ↓"
					}
				} else if isPat {
					newVerText += " (Pat)"
					newVerColor = tcell.ColorGreen
					if img.UpdateMajor != "" || img.UpdateMinor != "" {
						newVerText += " ↑"
					}
				} else {
					newVerColor = tcell.ColorWhite
				}
			}
		}
		r.table.SetCell(row, col, tview.NewTableCell(newVerText).SetTextColor(newVerColor).SetAlign(tview.AlignCenter))
		col++

		// Optional Column: Container
		if r.config.UI.ShowContainer {
			r.table.SetCell(row, col, tview.NewTableCell(img.ContainerName).SetTextColor(tcell.ColorWhite))
			col++
		}
	}
}

func truncateVersion(v string) string {
	if strings.HasPrefix(v, "sha256:") {
		if len(v) > 7+8 {
			return v[:15]
		}
		return v
	}
	// If it's a 64 character hex string
	if len(v) == 64 {
		isHex := true
		for _, c := range v {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isHex = false
				break
			}
		}
		if isHex {
			return v[:8]
		}
	}
	return v
}
