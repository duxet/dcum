package ui

import (
	"strings"
	"testing"

	"github.com/duxet/dcum/internal/config"
	"github.com/rivo/tview"
)

func TestFormatProgressBar(t *testing.T) {
	tests := []struct {
		name            string
		current         int
		total           int
		width           int
		expectedPercent string
		expectedCount   string
	}{
		{
			name:            "zero total",
			current:         0,
			total:           0,
			width:           10,
			expectedPercent: "100%",
			expectedCount:   "(1/1)",
		},
		{
			name:            "start 0 of 10",
			current:         0,
			total:           10,
			width:           20,
			expectedPercent: "  0%",
			expectedCount:   "(0/10)",
		},
		{
			name:            "half 5 of 10",
			current:         5,
			total:           10,
			width:           20,
			expectedPercent: " 50%",
			expectedCount:   "(5/10)",
		},
		{
			name:            "done 10 of 10",
			current:         10,
			total:           10,
			width:           20,
			expectedPercent: "100%",
			expectedCount:   "(10/10)",
		},
		{
			name:            "current exceeds total clamped",
			current:         15,
			total:           10,
			width:           20,
			expectedPercent: "100%",
			expectedCount:   "(10/10)",
		},
		{
			name:            "negative current clamped",
			current:         -5,
			total:           10,
			width:           20,
			expectedPercent: "  0%",
			expectedCount:   "(0/10)",
		},
		{
			name:            "default width when zero or negative",
			current:         3,
			total:           10,
			width:           0,
			expectedPercent: " 30%",
			expectedCount:   "(3/10)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatProgressBar(tt.current, tt.total, tt.width)
			if !strings.HasPrefix(result, "Fetching latest versions:") {
				t.Errorf("unexpected prefix: %s", result)
			}
			if !strings.Contains(result, tt.expectedPercent) {
				t.Errorf("expected percent %q in result %q", tt.expectedPercent, result)
			}
			if !strings.Contains(result, tt.expectedCount) {
				t.Errorf("expected count %q in result %q", tt.expectedCount, result)
			}

			// Verify TaggedStringWidth does not panic and parses tags properly
			w := tview.TaggedStringWidth(result)
			if w <= 0 {
				t.Errorf("expected positive width, got %d", w)
			}
		})
	}
}

func TestRoot_SetLayout(t *testing.T) {
	cfg := &config.Config{}
	r := NewRoot(cfg)

	if r.progressBar == nil {
		t.Fatal("expected progressBar to be initialized")
	}

	// Calling setLayout(true) and setLayout(false) should succeed without error
	r.setLayout(true)
	r.setLayout(false)
}

func TestRoot_UpdateProgressBar(t *testing.T) {
	cfg := &config.Config{}
	r := NewRoot(cfg)

	r.updateProgressBar(5, 10)
	text := r.progressBar.GetText(false)
	if !strings.Contains(text, "50%") {
		t.Errorf("expected 50%% in progress bar text, got: %s", text)
	}
	if !strings.Contains(text, "(5/10)") {
		t.Errorf("expected (5/10) in progress bar text, got: %s", text)
	}
}
