package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_ExcludePatterns(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test directories
	dirs := []string{
		"node_modules/project",
		".git/objects",
		"vendor/lib",
		"valid/subdir",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	// Create test docker-compose files
	composeContent := `version: '3'
services:
  test:
    image: nginx:1.21.6
`

	files := map[string]string{
		"node_modules/project/docker-compose.yml": composeContent,
		".git/objects/docker-compose.yml":         composeContent,
		"vendor/lib/docker-compose.yml":           composeContent,
		"valid/subdir/docker-compose.yml":         composeContent,
		"docker-compose.yml":                      composeContent,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Test with exclusion patterns
	excludePatterns := []string{
		"**/node_modules/**",
		"**/.git/**",
		"**/vendor/**",
	}

	scanner := NewScanner(excludePatterns)
	images, err := scanner.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should only find 2 compose files (root and valid/subdir)
	// Each has 1 service, so 2 images total
	if len(images) != 2 {
		t.Errorf("Expected 2 images, got %d", len(images))
		for _, img := range images {
			t.Logf("Found image in: %s", img.FilePath)
		}
	}

	// Verify none of the excluded paths are in results
	for _, img := range images {
		relPath, _ := filepath.Rel(tmpDir, img.FilePath)
		if filepath.HasPrefix(relPath, "node_modules") ||
			filepath.HasPrefix(relPath, ".git") ||
			filepath.HasPrefix(relPath, "vendor") {
			t.Errorf("Found image in excluded path: %s", relPath)
		}
	}
}

func TestScanner_NoExclusions(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test directories
	dirs := []string{
		"node_modules/project",
		"valid/subdir",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}
	}

	// Create test docker-compose files
	composeContent := `version: '3'
services:
  test:
    image: nginx:1.21.6
`

	files := map[string]string{
		"node_modules/project/docker-compose.yml": composeContent,
		"valid/subdir/docker-compose.yml":         composeContent,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", path, err)
		}
	}

	// Test without exclusion patterns
	scanner := NewScanner([]string{})
	images, err := scanner.Scan(tmpDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should find both compose files (2 images total)
	// Note: .git is still excluded by default in the scanner code
	if len(images) < 2 {
		t.Errorf("Expected at least 2 images, got %d", len(images))
	}
}
