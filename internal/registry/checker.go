package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Checker checks for updates in container registries.
type Checker struct {
	cacheMu   sync.Mutex
	cache     map[string]CachedCandidates
	cachePath string
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

// GetUpdateCandidates returns the latest patch, minor, and major versions for a given image.
func (c *Checker) GetUpdateCandidates(imageName, currentVersion string, includeRegex string, excludeRegex string, forceRefresh bool) (UpdateCandidates, error) {
	cacheKey := imageName + "|" + currentVersion + "|" + includeRegex + "|" + excludeRegex

	if !forceRefresh {
		c.cacheMu.Lock()
		if val, ok := c.cache[cacheKey]; ok {
			// Basic validity check? Maybe expiry? For now assume valid until refreshed.
			c.cacheMu.Unlock()
			return val.Candidates, nil
		}
		c.cacheMu.Unlock()
	}

	var candidates UpdateCandidates

	// Parse current version
	currentV, err := semver.NewVersion(currentVersion)
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

	var parsedVersions []*semver.Version
	for _, tag := range tags {
		if includeFilter != nil && !includeFilter.MatchString(tag) {
			continue
		}
		if excludeFilter != nil && excludeFilter.MatchString(tag) {
			continue
		}

		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		// If custom regex matches, we should allow pre-releases if they are valid semver
		// This is because things like "-alpine" might be considered pre-release metadata by semver parser
		if includeFilter == nil && v.Prerelease() != "" {
			continue
		}
		parsedVersions = append(parsedVersions, v)
	}

	sort.Sort(semver.Collection(parsedVersions))

	// Find candidates
	for _, v := range parsedVersions {
		if v.LessThan(currentV) || v.Equal(currentV) {
			continue
		}

		if v.Major() > currentV.Major() {
			candidates.Major = v.Original() // Always take the highest seen so far logic?
			// Since sorted ascending, the last assignment will be the highest.
		} else if v.Minor() > currentV.Minor() {
			candidates.Minor = v.Original()
		} else if v.Patch() > currentV.Patch() {
			candidates.Patch = v.Original()
		}
	}

	// If we found a major update, but no minor/patch, that's fine.
	// But usually we want:
	// Patch: Highest version with same Major.Minor
	// Minor: Highest version with same Major, but higher Minor
	// Major: Highest version with higher Major

	// Re-iterate properly to find specific candidates
	var bestPatch, bestMinor, bestMajor *semver.Version

	for _, v := range parsedVersions {
		if v.LessThan(currentV) || v.Equal(currentV) {
			continue
		}

		if v.Major() == currentV.Major() && v.Minor() == currentV.Minor() {
			bestPatch = v
		} else if v.Major() == currentV.Major() && v.Minor() > currentV.Minor() {
			bestMinor = v
		} else if v.Major() > currentV.Major() {
			bestMajor = v
		}
	}

	if bestPatch != nil {
		candidates.Patch = bestPatch.Original()
	}
	if bestMinor != nil {
		candidates.Minor = bestMinor.Original()
	}
	if bestMajor != nil {
		candidates.Major = bestMajor.Original()
	}

	c.cacheMu.Lock()
	c.cache[cacheKey] = CachedCandidates{
		Candidates: candidates,
		Timestamp:  time.Now(),
	}
	c.cacheMu.Unlock()

	// Save to disk asynchronously/immediately
	// Since we are inside a goroutine from the UI (mostly), this is fine.
	// But saving on EVERY update might be heavy if many updates happen at once.
	// However, mutex protects the map. The saveCache also locks mutex.
	// We should probably save outside lock.
	c.saveCache()

	return candidates, nil
}
