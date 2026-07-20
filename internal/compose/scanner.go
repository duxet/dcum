package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/v2/cli"
)

// UpdateImages updates the docker-compose files (or .env files) with new image versions.
func (s *Scanner) UpdateImages(images []ContainerImage) error {
	type FileUpdate struct {
		IsEnv      bool
		EnvVarName string
		Image      ContainerImage
	}

	updatesByFile := make(map[string][]FileUpdate)
	for _, img := range images {
		if img.NewVersion == "" {
			continue
		}

		if img.EnvVarName != "" && img.EnvFilePath != "" {
			updatesByFile[img.EnvFilePath] = append(updatesByFile[img.EnvFilePath], FileUpdate{
				IsEnv:      true,
				EnvVarName: img.EnvVarName,
				Image:      img,
			})
		} else {
			updatesByFile[img.FilePath] = append(updatesByFile[img.FilePath], FileUpdate{
				IsEnv: false,
				Image: img,
			})
		}
	}

	for filePath, updates := range updatesByFile {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file %s: %w", filePath, err)
		}

		strContent := string(content)

		if updates[0].IsEnv {
			for _, u := range updates {
				var updated bool
				strContent, updated = updateEnvVar(strContent, u.EnvVarName, u.Image.NewVersion)
				if !updated {
					return fmt.Errorf("variable %s not found or could not be updated in %s", u.EnvVarName, filePath)
				}
			}
		} else {
			for _, u := range updates {
				update := u.Image
				sep := ":"
				if strings.HasPrefix(update.CurrentVersion, "sha256:") {
					sep = "@"
				}
				oldImageStr := fmt.Sprintf("%s%s%s", update.ImageName, sep, update.CurrentVersion)

				newSep := ":"
				if strings.HasPrefix(update.NewVersion, "sha256:") {
					newSep = "@"
				}
				newImageStr := fmt.Sprintf("%s%s%s", update.ImageName, newSep, update.NewVersion)

				strContent = strings.Replace(strContent, oldImageStr, newImageStr, -1)
			}
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
	EnvVarName     string // Name of the environment variable (e.g. GRAYLOG_VERSION) if the version is defined in a .env file
	EnvFilePath    string // Path to the .env file containing the version definition
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

var varRegex = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?:[^}]*)?\}|\$([a-zA-Z_][a-zA-Z0-9_]*)`)

func extractVars(s string) []string {
	matches := varRegex.FindAllStringSubmatch(s, -1)
	var vars []string
	for _, m := range matches {
		if m[1] != "" {
			vars = append(vars, m[1])
		} else if m[2] != "" {
			vars = append(vars, m[2])
		}
	}
	return vars
}

func hasEnvVar(envFilePath string, varName string) bool {
	content, err := os.ReadFile(envFilePath)
	if err != nil {
		return false
	}
	pattern := fmt.Sprintf(`(?m)^(\s*(?:export\s+)?%s\s*=)`, regexp.QuoteMeta(varName))
	matched, _ := regexp.Match(pattern, content)
	return matched
}

func updateEnvVar(content string, varName string, newValue string) (string, bool) {
	lines := strings.Split(content, "\n")
	updated := false

	pattern := fmt.Sprintf(`^(\s*(?:export\s+)?%s\s*=\s*)(['"]?)(.*?)(['"]?)(\s*(?:#.*)?)$`, regexp.QuoteMeta(varName))
	re, err := regexp.Compile(pattern)
	if err != nil {
		return content, false
	}

	for i, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) == 6 {
			openQuote := matches[2]
			closeQuote := matches[4]
			if openQuote == closeQuote {
				lines[i] = fmt.Sprintf("%s%s%s%s%s", matches[1], openQuote, newValue, closeQuote, matches[5])
				updated = true
			}
		}
	}

	return strings.Join(lines, "\n"), updated
}

func splitImageAndTag(imageStr string) (name string, tag string) {
	braceDepth := 0
	lastAt := -1
	lastColon := -1
	lastSlash := -1

	runes := []rune(imageStr)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '{' {
			braceDepth++
		} else if c == '}' {
			braceDepth--
		} else if braceDepth == 0 {
			if c == '@' {
				lastAt = i
			} else if c == ':' {
				lastColon = i
			} else if c == '/' {
				lastSlash = i
			}
		}
	}

	if lastAt > lastSlash {
		return string(runes[:lastAt]), string(runes[lastAt+1:])
	}
	if lastColon > lastSlash {
		return string(runes[:lastColon]), string(runes[lastColon+1:])
	}
	return imageStr, ""
}

func parseComposeFile(path string) ([]ContainerImage, error) {
	// 1. Load project WITH interpolation (correctly loading .env)
	opts, err := cli.NewProjectOptions([]string{path}, cli.WithEnvFiles(), cli.WithDotEnv, cli.WithOsEnv)
	if err != nil {
		return nil, err
	}

	project, err := cli.ProjectFromOptions(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	// 2. Load project WITHOUT interpolation to find environment variable definitions in image tags
	rawImages := make(map[string]string)
	optsNoInt, err := cli.NewProjectOptions([]string{path}, cli.WithInterpolation(false))
	if err == nil {
		if projectNoInt, err := cli.ProjectFromOptions(context.Background(), optsNoInt); err == nil {
			for _, service := range projectNoInt.Services {
				rawImages[service.Name] = service.Image
			}
		}
	}

	var images []ContainerImage
	for _, service := range project.Services {
		imageName := service.Image
		if imageName == "" {
			continue
		}

		name, version := splitImageAndTag(imageName)
		if version == "" {
			version = "latest"
		}

		// Check if the version is defined via environment variable in the original file
		var envVarName, envFilePath string
		if rawImage, found := rawImages[service.Name]; found && rawImage != "" {
			_, rawTag := splitImageAndTag(rawImage)
			if rawTag != "" {
				vars := extractVars(rawTag)
				if len(vars) > 0 {
					vName := vars[0]
					dotEnvPath := filepath.Join(filepath.Dir(path), ".env")
					if hasEnvVar(dotEnvPath, vName) {
						envVarName = vName
						envFilePath = dotEnvPath
					}
				}
			}
		}

		images = append(images, ContainerImage{
			ServiceName:    service.Name,
			ContainerName:  service.ContainerName,
			ImageName:      name,
			CurrentVersion: version,
			Labels:         service.Labels,
			FilePath:       path,
			EnvVarName:     envVarName,
			EnvFilePath:    envFilePath,
		})
	}

	return images, nil
}
