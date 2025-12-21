package registry

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Checker checks for updates in container registries.
type Checker struct{}

func NewChecker() *Checker {
	return &Checker{}
}

// GetLatestVersion returns the latest semantic version for a given image that is greater than currentVersion.
func (c *Checker) GetLatestVersion(imageName, currentVersion string) (string, error) {
	// Parse current version to ensure it's semantic
	currentV, err := semver.NewVersion(currentVersion)
	if err != nil {
		// If current version is not semver (e.g. "latest", "stable", or distinct tag),
		// we likely can't suggest a semantic upgrade easily without more logic.
		return "", nil
	}

	repo, err := name.NewRepository(imageName)
	if err != nil {
		return "", fmt.Errorf("parsing repo name: %w", err)
	}

	// Fetch tags
	tags, err := remote.List(repo)
	if err != nil {
		return "", fmt.Errorf("listing tags: %w", err)
	}

	var parsedVersions []*semver.Version
	for _, tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue // Skip non-semver tags
		}
		// Only consider stable versions (not pre-releases) unless current is pre-release?
		// For now, let's include everything but maybe filter strictly later.
		// Usually we want to upgrade to stable.
		if v.Prerelease() != "" {
			continue
		}
		parsedVersions = append(parsedVersions, v)
	}

	// Sort versions
	sort.Sort(semver.Collection(parsedVersions))

	// Find the highest version greater than current
	// Since it's sorted, the last one is the highest.
	if len(parsedVersions) > 0 {
		latest := parsedVersions[len(parsedVersions)-1]
		if latest.GreaterThan(currentV) {
			return latest.Original(), nil
		}
	}

	return "", nil
}
