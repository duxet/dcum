package app

import (
	"fmt"
	"os"

	"github.com/duxet/dcum/internal/compose"
	"github.com/duxet/dcum/internal/ui"
)

// Run initializes and starts the application.
func Run() error {
	scanner := compose.NewScanner()
	
	// Start scanning from current directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fmt.Println("Scanning for docker-compose files...")
	images, err := scanner.Scan(wd)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(images) == 0 {
		fmt.Println("No docker-compose files found or no images found in them.")
		// We can still open the UI or just exit. 
		// For better UX, let's open UI but it will be empty.
		// Or maybe just tell the user. 
		// Let's print a message and exit for now, or just show empty table.
		// showing empty table is better for continuity.
	}

	appUI := ui.NewRoot()
	return appUI.Render(images)
}
