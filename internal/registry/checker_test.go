package registry

import (
	"testing"
)

func TestApplyTransform(t *testing.T) {
	testCases := []struct {
		name          string
		tag           string
		transformExpr string
		expected      string
	}{
		{
			name:          "Plex format",
			tag:           "1.43.2.10687-563d026ea",
			transformExpr: `^(\d+\.\d+\.\d+)\.\d+-[a-f0-9]+$$ => $$1`,
			expected:      "1.43.2",
		},
		{
			name:          "No match",
			tag:           "1.43.2",
			transformExpr: `^(\d+\.\d+\.\d+)\.\d+-[a-f0-9]+$$ => $$1`,
			expected:      "1.43.2",
		},
		{
			name:          "Empty transform expression",
			tag:           "1.43.2.10687-563d026ea",
			transformExpr: "",
			expected:      "1.43.2.10687-563d026ea",
		},
		{
			name:          "Invalid expression format",
			tag:           "1.43.2.10687-563d026ea",
			transformExpr: "invalid-expression",
			expected:      "1.43.2.10687-563d026ea",
		},
		{
			name:          "Alpine suffix strip",
			tag:           "3.15.0-alpine",
			transformExpr: `^(\d+\.\d+\.\d+)-alpine$$ => $$1`,
			expected:      "3.15.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := applyTransform(tc.tag, tc.transformExpr)
			if output != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, output)
			}
		})
	}
}
