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

// AutoSelectMax returns the highest update level that may be selected
// automatically for a service with the given labels. Without the label every
// level is allowed. An unrecognized value disables automatic selection, so a
// typo never results in an unexpected major bump.
func AutoSelectMax(labels map[string]string) UpdateLevel {
	val, ok := labels[AutoSelectMaxLabel]
	if !ok {
		return UpdateLevelMajor
	}
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "patch":
		return UpdateLevelPatch
	case "minor":
		return UpdateLevelMinor
	case "major":
		return UpdateLevelMajor
	default:
		return UpdateLevelNone
	}
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
