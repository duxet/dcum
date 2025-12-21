package compose

import (
	"path/filepath"
	"testing"
)

func TestScanner_Scan(t *testing.T) {
	s := NewScanner()
	wd, _ := filepath.Abs("../../") // Go to root of repo
	
	images, err := s.Scan(wd)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	found := false
	for _, img := range images {
		if img.ImageName == "nginx" && img.CurrentVersion == "1.21.6" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find nginx:1.21.6, but got: %v", images)
	}
}
