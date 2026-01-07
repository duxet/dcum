package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
)

// UpdateImages updates the docker-compose files with new image versions.
func (s *Scanner) UpdateImages(images []ContainerImage) error {
	// Group updates by file
	updatesByFile := make(map[string][]ContainerImage)
	for _, img := range images {
		if img.NewVersion != "" {
			updatesByFile[img.FilePath] = append(updatesByFile[img.FilePath], img)
		}
	}

	for filePath, updates := range updatesByFile {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", filePath, err)
		}

		strContent := string(content)

		for _, update := range updates {
			// Naive replacement: finding "image: name:version" and replacing it.
			// This might be risky if whitespace varies or if same image is used multiple times with different tags (unlikely for exact string match).
			// Better is to replace the specific instance.

			// Construct old and new strings
			oldImageStr := fmt.Sprintf("%s:%s", update.ImageName, update.CurrentVersion)
			newImageStr := fmt.Sprintf("%s:%s", update.ImageName, update.NewVersion)

			// To be safer, we should probably use the line we found initially?
			// But we didn't store line numbers.
			// Let's assume standard "image: name:tag" format for now, or just replace the substring.

			// This will replace ALL occurrences of that image:tag in the file.
			// Usually acceptable for a simple tool, but strictly maybe user wants to update only one service?
			// Given our UI lists service, we effectively update "Service" entry.
			// But if two services use same image:tag, this replace updates both.
			// To fix this accurately without line numbers, we'd need to re-parse or use AST.
			// For this task, let's Stick to simple ReplaceAll but warn/document.

			strContent = strings.Replace(strContent, oldImageStr, newImageStr, -1)
		}

		if err := os.WriteFile(filePath, []byte(strContent), 0644); err != nil {
			return fmt.Errorf("writing file %s: %w", filePath, err)
		}
	}
	return nil
}

// ContainerImage represents a container image found in a compose file.
type ContainerImage struct {
	ServiceName    string
	ContainerName  string
	ImageName      string
	CurrentVersion string
	NewVersion     string
	UpdatePatch    string
	UpdateMinor    string
	UpdateMajor    string
	Labels         map[string]string
	FilePath       string
}

// Scanner scans directories for docker-compose files.
type Scanner struct {
	excludePatterns []string
}

func NewScanner(excludePatterns []string) *Scanner {
	return &Scanner{
		excludePatterns: excludePatterns,
	}
}

// Scan walks the directory tree and finds all docker-compose files and their images.
func (s *Scanner) Scan(rootDir string) ([]ContainerImage, error) {
	var images []ContainerImage

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for pattern matching
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			relPath = path
		}

		// Check if path matches any exclusion pattern
		if s.shouldExclude(relPath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir // Skip hidden directories like .git
			}
			return nil
		}

		if isComposeFile(info.Name()) {
			imgs, err := parseComposeFile(path)
			if err != nil {
				// We log scanning errors but continue scanning other files
				// In a real app we might want to collect these errors
				fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", path, err)
				return nil
			}
			images = append(images, imgs...)
		}
		return nil
	})

	return images, err
}

// shouldExclude checks if a path matches any exclusion pattern.
func (s *Scanner) shouldExclude(path string, isDir bool) bool {
	for _, pattern := range s.excludePatterns {
		// For directories, also check with trailing slash
		if isDir {
			matched, err := filepath.Match(pattern, path+"/")
			if err == nil && matched {
				return true
			}
		}

		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}

		// Also check if any parent directory matches
		// This handles patterns like "**/node_modules/**"
		parts := strings.Split(pattern, "/")
		pathParts := strings.Split(filepath.ToSlash(path), "/")

		// Simple glob matching for ** patterns
		if len(parts) > 0 {
			for i := range pathParts {
				subPath := strings.Join(pathParts[:i+1], "/")
				matched, err := filepath.Match(pattern, subPath)
				if err == nil && matched {
					return true
				}

				// Handle ** wildcard by checking if pattern contains the path segment
				for _, part := range parts {
					if part != "**" && part != "*" {
						for _, pathPart := range pathParts {
							if matched, _ := filepath.Match(part, pathPart); matched {
								// Check if this is part of a ** pattern
								if strings.Contains(pattern, "**") && strings.Contains(pattern, part) {
									return true
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

func isComposeFile(filename string) bool {
	return filename == "docker-compose.yml" || filename == "docker-compose.yaml" ||
		filename == "compose.yml" || filename == "compose.yaml"
}

func parseComposeFile(path string) ([]ContainerImage, error) {
	// compose-go needs a ProjectOptions to load
	opts, err := cli.NewProjectOptions([]string{path}, cli.WithDotEnv, cli.WithOsEnv)
	if err != nil {
		return nil, err
	}

	project, err := cli.ProjectFromOptions(context.Background(), opts)
	if err != nil {
		// Fallback for simple cases if full loading fails (e.g. missing vars)
		// For now let's try to load just the config without interpolation if possible,
		// but cli.ProjectFromOptions is the standard way.
		// If it fails, it might be due to missing env vars.
		// Let's try to ignore interpolation errors if possible, but compose-go is strict.

		// Alternative: Manually load using loader if CLI fails?
		// Let's stick to standard scan for now.
		return nil, err
	}

	var images []ContainerImage
	for _, service := range project.Services {
		imageName := service.Image
		if imageName == "" {
			continue
		}

		parts := strings.Split(imageName, ":")
		name := parts[0]
		version := "latest"
		if len(parts) > 1 {
			version = parts[1]
		}

		// Handle images with digest (e.g. @sha256:...) - simplification for now
		if strings.Contains(version, "@") {
			// complex parsing, let's keep it simple for now and store what we have
		}

		images = append(images, ContainerImage{
			ServiceName:    service.Name,
			ContainerName:  service.ContainerName,
			ImageName:      name,
			CurrentVersion: version,
			Labels:         service.Labels,
			FilePath:       path,
		})
	}

	return images, nil
}
