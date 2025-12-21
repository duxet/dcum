package app

import (
	"fmt"
	"os"

	"github.com/duxet/dcum/internal/compose"
	"github.com/duxet/dcum/internal/registry"
	"github.com/duxet/dcum/internal/ui"
)

// Run initializes and starts the application.
func Run() error {
	scanner := compose.NewScanner()
	checker := registry.NewChecker()

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
	} else {
		fmt.Println("Checking for updates...")
		for i := range images {
			fmt.Printf("Checking %s:%s...\n", images[i].ImageName, images[i].CurrentVersion)
			newVer, err := checker.GetLatestVersion(images[i].ImageName, images[i].CurrentVersion)
			if err != nil {
				// Log error but continue
				fmt.Fprintf(os.Stderr, "Failed to check %s: %v\n", images[i].ImageName, err)
				continue
			}
			if newVer != "" {
				images[i].NewVersion = newVer
			}
		}
	}

	appUI := ui.NewRoot()
	return appUI.Render(images)
}
