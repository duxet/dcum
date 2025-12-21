package registry

import (
	"fmt"
	"regexp"
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

// UpdateCandidates holds potential version upgrades.
type UpdateCandidates struct {
	Patch string
	Minor string
	Major string
}

// GetUpdateCandidates returns the latest patch, minor, and major versions for a given image.
func (c *Checker) GetUpdateCandidates(imageName, currentVersion string, tagRegex string) (UpdateCandidates, error) {
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

	var filters *regexp.Regexp
	if tagRegex != "" {
		r, err := regexp.Compile(tagRegex)
		if err != nil {
			// If regex is invalid, maybe just ignore or return error?
			// returning error seems safer.
			return candidates, fmt.Errorf("invalid regex %s: %w", tagRegex, err)
		}
		filters = r
	}

	var parsedVersions []*semver.Version
	for _, tag := range tags {
		if filters != nil && !filters.MatchString(tag) {
			continue
		}

		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		// If custom regex matches, we should allow pre-releases if they are valid semver
		// This is because things like "-alpine" might be considered pre-release metadata by semver parser
		if filters == nil && v.Prerelease() != "" {
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

	return candidates, nil
}
