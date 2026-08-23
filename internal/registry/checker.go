package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/singleflight"
)

// Checker checks for updates in container registries.
type Checker struct {
	cacheMu   sync.Mutex
	cache     map[string]CachedCandidates
	cachePath string
	sf        singleflight.Group
}

type CachedCandidates struct {
	Candidates UpdateCandidates
	Timestamp  time.Time
}

func NewChecker() *Checker {
	c := &Checker{
		cache: make(map[string]CachedCandidates),
	}

	if cacheDir, err := os.UserCacheDir(); err == nil {
		dcumCacheDir := filepath.Join(cacheDir, "dcum")
		if err := os.MkdirAll(dcumCacheDir, 0755); err == nil {
			c.cachePath = filepath.Join(dcumCacheDir, "registry_cache.json")
			c.loadCache()
		}
	}

	return c
}

func (c *Checker) loadCache() {
	if c.cachePath == "" {
		return
	}
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return // Ignore error, start fresh
	}
	var loaded map[string]CachedCandidates
	if err := json.Unmarshal(data, &loaded); err == nil {
		c.cache = loaded
	}
}

func (c *Checker) saveCache() {
	if c.cachePath == "" {
		return
	}
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err == nil {
		_ = os.WriteFile(c.cachePath, data, 0644)
	}
}

// UpdateCandidates holds potential version upgrades.
type UpdateCandidates struct {
	Patch string
	Minor string
	Major string
}

func applyTransform(tag string, transformExpr string) string {
	if transformExpr == "" {
		return tag
	}

	parts := strings.Split(transformExpr, "=>")
	if len(parts) != 2 {
		return tag
	}

	pattern := strings.TrimSpace(parts[0])
	replacement := strings.TrimSpace(parts[1])

	// Replace $$ with $
	replacement = strings.ReplaceAll(replacement, "$$", "$")

	re, err := regexp.Compile(pattern)
	if err != nil {
		return tag
	}

	if re.MatchString(tag) {
		return re.ReplaceAllString(tag, replacement)
	}
	return tag
}

// GetUpdateCandidates returns the latest patch, minor, and major versions for a given image.
func (c *Checker) GetUpdateCandidates(imageName, currentVersion string, includeRegex, excludeRegex, transformExpr string, forceRefresh bool) (UpdateCandidates, error) {
	cacheKey := imageName + "|" + currentVersion + "|" + includeRegex + "|" + excludeRegex + "|" + transformExpr

	if !forceRefresh {
		c.cacheMu.Lock()
		if val, ok := c.cache[cacheKey]; ok {
			c.cacheMu.Unlock()
			return val.Candidates, nil
		}
		c.cacheMu.Unlock()
	}

	val, err, _ := c.sf.Do(cacheKey, func() (interface{}, error) {
		if !forceRefresh {
			c.cacheMu.Lock()
			if val, ok := c.cache[cacheKey]; ok {
				c.cacheMu.Unlock()
				return val.Candidates, nil
			}
			c.cacheMu.Unlock()
		}

		var candidates UpdateCandidates

		// Apply transform to current version if specified
		transformedCurrent := applyTransform(currentVersion, transformExpr)
		currentV, err := semver.NewVersion(transformedCurrent)
		if err != nil {
			return candidates, nil
		}

		repo, err := name.NewRepository(imageName)
		if err != nil {
			return candidates, fmt.Errorf("parsing repo name: %w", err)
		}

		// Fetch tags
		tags, err := remote.List(repo)
		if err != nil {
			return candidates, fmt.Errorf("listing tags: %w", err)
		}

		var includeFilter *regexp.Regexp
		if includeRegex != "" {
			r, err := regexp.Compile(includeRegex)
			if err != nil {
				return candidates, fmt.Errorf("invalid include regex %s: %w", includeRegex, err)
			}
			includeFilter = r
		}

		var excludeFilter *regexp.Regexp
		if excludeRegex != "" {
			r, err := regexp.Compile(excludeRegex)
			if err != nil {
				return candidates, fmt.Errorf("invalid exclude regex %s: %w", excludeRegex, err)
			}
			excludeFilter = r
		}

		type versionMapping struct {
			Original string
			Parsed   *semver.Version
		}

		var parsedVersions []versionMapping
		for _, tag := range tags {
			if includeFilter != nil && !includeFilter.MatchString(tag) {
				continue
			}
			if excludeFilter != nil && excludeFilter.MatchString(tag) {
				continue
			}

			transformedTag := applyTransform(tag, transformExpr)
			v, err := semver.NewVersion(transformedTag)
			if err != nil {
				continue
			}

			// If custom regex matches, we should allow pre-releases if they are valid semver
			if includeFilter == nil && v.Prerelease() != "" {
				continue
			}
			parsedVersions = append(parsedVersions, versionMapping{
				Original: tag,
				Parsed:   v,
			})
		}

		sort.Slice(parsedVersions, func(i, j int) bool {
			return parsedVersions[i].Parsed.LessThan(parsedVersions[j].Parsed)
		})

		var bestPatch, bestMinor, bestMajor versionMapping
		hasPatch, hasMinor, hasMajor := false, false, false

		for _, v := range parsedVersions {
			if v.Parsed.LessThan(currentV) || v.Parsed.Equal(currentV) {
				continue
			}

			if v.Parsed.Major() == currentV.Major() && v.Parsed.Minor() == currentV.Minor() {
				bestPatch = v
				hasPatch = true
			} else if v.Parsed.Major() == currentV.Major() && v.Parsed.Minor() > currentV.Minor() {
				bestMinor = v
				hasMinor = true
			} else if v.Parsed.Major() > currentV.Major() {
				bestMajor = v
				hasMajor = true
			}
		}

		if hasPatch {
			candidates.Patch = bestPatch.Original
		}
		if hasMinor {
			candidates.Minor = bestMinor.Original
		}
		if hasMajor {
			candidates.Major = bestMajor.Original
		}

		c.cacheMu.Lock()
		c.cache[cacheKey] = CachedCandidates{
			Candidates: candidates,
			Timestamp:  time.Now(),
		}
		c.cacheMu.Unlock()

		c.saveCache()

		return candidates, nil
	})

	if err != nil {
		return UpdateCandidates{}, err
	}
	return val.(UpdateCandidates), nil
}
