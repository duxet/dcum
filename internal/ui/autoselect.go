package ui

import (
	"strings"

	"github.com/duxet/dcum/internal/registry"
)

// AutoSelectMaxLabel limits the highest update level that is selected
// automatically after an update check. Higher levels stay available and can
// still be picked manually by cycling versions.
const AutoSelectMaxLabel = "dcum.autoselect.max"

// UpdateLevel orders update kinds from least to most disruptive.
type UpdateLevel int

const (
	UpdateLevelNone UpdateLevel = iota
	UpdateLevelPatch
	UpdateLevelMinor
	UpdateLevelMajor
)

// ParseUpdateLevel parses "patch", "minor" or "major" (case-insensitive).
func ParseUpdateLevel(s string) (UpdateLevel, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "patch":
		return UpdateLevelPatch, true
	case "minor":
		return UpdateLevelMinor, true
	case "major":
		return UpdateLevelMajor, true
	default:
		return UpdateLevelNone, false
	}
}

// AutoSelectMax returns the highest update level that may be selected
// automatically for a service with the given labels. Without the label the
// fallback (from the configuration file) applies. An unrecognized label value
// disables automatic selection, so a typo never results in an unexpected
// major bump.
func AutoSelectMax(labels map[string]string, fallback UpdateLevel) UpdateLevel {
	val, ok := labels[AutoSelectMaxLabel]
	if !ok {
		return fallback
	}
	level, _ := ParseUpdateLevel(val)
	return level
}

// DefaultVersion picks the version to select automatically, preferring
// Patch > Minor > Major and skipping levels above maxLevel. It returns an
// empty string when no candidate is allowed.
func DefaultVersion(candidates registry.UpdateCandidates, maxLevel UpdateLevel) string {
	switch {
	case candidates.Patch != "" && maxLevel >= UpdateLevelPatch:
		return candidates.Patch
	case candidates.Minor != "" && maxLevel >= UpdateLevelMinor:
		return candidates.Minor
	case candidates.Major != "" && maxLevel >= UpdateLevelMajor:
		return candidates.Major
	}
	return ""
}
