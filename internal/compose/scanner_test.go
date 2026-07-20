package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanner_Scan(t *testing.T) {
	s := NewScanner([]string{})     // No exclusions for test
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

func TestScanner_ScanAndUpdateEnv(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dcum-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	composePath := filepath.Join(tempDir, "docker-compose.yaml")
	composeContent := `services:
  graylog:
    container_name: graylog
    image: graylog/graylog:${GRAYLOG_VERSION:-latest}
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose file: %v", err)
	}

	envPath := filepath.Join(tempDir, ".env")
	envContent := `# Some initial comments
GRAYLOG_VERSION=7.1.5
GRAYLOG_PASSWORD_SECRET=xyz # secret
`
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write env file: %v", err)
	}

	s := NewScanner([]string{})
	images, err := s.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var targetImg *ContainerImage
	for i := range images {
		if images[i].ServiceName == "graylog" {
			targetImg = &images[i]
			break
		}
	}

	if targetImg == nil {
		t.Fatalf("Expected to find graylog service in scanned images, but got: %v", images)
	}

	if targetImg.CurrentVersion != "7.1.5" {
		t.Errorf("Expected current version 7.1.5, got: %s", targetImg.CurrentVersion)
	}

	if targetImg.EnvVarName != "GRAYLOG_VERSION" {
		t.Errorf("Expected EnvVarName 'GRAYLOG_VERSION', got: %s", targetImg.EnvVarName)
	}

	if targetImg.EnvFilePath != envPath {
		t.Errorf("Expected EnvFilePath %q, got: %q", envPath, targetImg.EnvFilePath)
	}

	// Update image version
	targetImg.NewVersion = "7.2.0"
	if err := s.UpdateImages([]ContainerImage{*targetImg}); err != nil {
		t.Fatalf("UpdateImages failed: %v", err)
	}

	// Verify updated env content
	updatedEnvBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("Failed to read updated env file: %v", err)
	}

	expectedEnvContent := `# Some initial comments
GRAYLOG_VERSION=7.2.0
GRAYLOG_PASSWORD_SECRET=xyz # secret
`
	if string(updatedEnvBytes) != expectedEnvContent {
		t.Errorf("Env content mismatch.\nExpected:\n%s\nGot:\n%s", expectedEnvContent, string(updatedEnvBytes))
	}
}
