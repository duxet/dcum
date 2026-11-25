package ui

import (
	"testing"

	"github.com/duxet/dcum/internal/registry"
)

func TestAutoSelectMax(t *testing.T) {
	testCases := []struct {
		name     string
		labels   map[string]string
		expected UpdateLevel
	}{
		{name: "Nil labels", labels: nil, expected: UpdateLevelMajor},
		{name: "Label missing", labels: map[string]string{"wud.tag.include": "^15\\."}, expected: UpdateLevelMajor},
		{name: "Patch", labels: map[string]string{AutoSelectMaxLabel: "patch"}, expected: UpdateLevelPatch},
		{name: "Minor", labels: map[string]string{AutoSelectMaxLabel: "minor"}, expected: UpdateLevelMinor},
		{name: "Major", labels: map[string]string{AutoSelectMaxLabel: "major"}, expected: UpdateLevelMajor},
		{name: "Case and whitespace", labels: map[string]string{AutoSelectMaxLabel: " Minor "}, expected: UpdateLevelMinor},
		{name: "Unknown value", labels: map[string]string{AutoSelectMaxLabel: "minro"}, expected: UpdateLevelNone},
		{name: "Empty value", labels: map[string]string{AutoSelectMaxLabel: ""}, expected: UpdateLevelNone},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AutoSelectMax(tc.labels); got != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, got)
			}
		})
	}
}

func TestDefaultVersion(t *testing.T) {
	all := registry.UpdateCandidates{Patch: "16.4", Minor: "16.6", Major: "17.2"}
	majorOnly := registry.UpdateCandidates{Major: "17.2"}
	minorAndMajor := registry.UpdateCandidates{Minor: "16.6", Major: "17.2"}

	testCases := []struct {
		name       string
		candidates registry.UpdateCandidates
		maxLevel   UpdateLevel
		expected   string
	}{
		{name: "No candidates", candidates: registry.UpdateCandidates{}, maxLevel: UpdateLevelMajor, expected: ""},
		{name: "Prefers patch", candidates: all, maxLevel: UpdateLevelMajor, expected: "16.4"},
		{name: "Falls back to minor", candidates: minorAndMajor, maxLevel: UpdateLevelMajor, expected: "16.6"},
		{name: "Falls back to major", candidates: majorOnly, maxLevel: UpdateLevelMajor, expected: "17.2"},
		{name: "Minor cap skips major", candidates: majorOnly, maxLevel: UpdateLevelMinor, expected: ""},
		{name: "Minor cap allows minor", candidates: minorAndMajor, maxLevel: UpdateLevelMinor, expected: "16.6"},
		{name: "Patch cap allows patch", candidates: all, maxLevel: UpdateLevelPatch, expected: "16.4"},
		{name: "Patch cap skips minor", candidates: minorAndMajor, maxLevel: UpdateLevelPatch, expected: ""},
		{name: "None selects nothing", candidates: all, maxLevel: UpdateLevelNone, expected: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DefaultVersion(tc.candidates, tc.maxLevel); got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
