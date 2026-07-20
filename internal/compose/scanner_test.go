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

func TestScanner_ScanAndUpdateEnvNested(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dcum-nested-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Setup service-a
	dirA := filepath.Join(tempDir, "service-a")
	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatalf("Failed to create service-a dir: %v", err)
	}
	composeContentA := `services:
  app-a:
    image: image-a:${A_VERSION:-latest}
`
	if err := os.WriteFile(filepath.Join(dirA, "docker-compose.yaml"), []byte(composeContentA), 0644); err != nil {
		t.Fatalf("Failed to write compose-a: %v", err)
	}
	envContentA := `A_VERSION=1.1.0
`
	if err := os.WriteFile(filepath.Join(dirA, ".env"), []byte(envContentA), 0644); err != nil {
		t.Fatalf("Failed to write env-a: %v", err)
	}

	// Setup service-b
	dirB := filepath.Join(tempDir, "service-b")
	if err := os.MkdirAll(dirB, 0755); err != nil {
		t.Fatalf("Failed to create service-b dir: %v", err)
	}
	composeContentB := `services:
  app-b:
    image: image-b:${B_VERSION:-latest}
`
	if err := os.WriteFile(filepath.Join(dirB, "docker-compose.yaml"), []byte(composeContentB), 0644); err != nil {
		t.Fatalf("Failed to write compose-b: %v", err)
	}
	envContentB := `B_VERSION=2.2.0
`
	if err := os.WriteFile(filepath.Join(dirB, ".env"), []byte(envContentB), 0644); err != nil {
		t.Fatalf("Failed to write env-b: %v", err)
	}

	s := NewScanner([]string{})
	images, err := s.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(images) != 2 {
		t.Fatalf("Expected 2 images, got %d: %v", len(images), images)
	}

	var imgA, imgB *ContainerImage
	for i := range images {
		if images[i].ServiceName == "app-a" {
			imgA = &images[i]
		} else if images[i].ServiceName == "app-b" {
			imgB = &images[i]
		}
	}

	if imgA == nil || imgB == nil {
		t.Fatalf("Failed to find app-a or app-b in scanned images: %v", images)
	}

	if imgA.CurrentVersion != "1.1.0" || imgA.EnvVarName != "A_VERSION" || imgA.EnvFilePath != filepath.Join(dirA, ".env") {
		t.Errorf("Image A metadata mismatch: %+v", imgA)
	}

	if imgB.CurrentVersion != "2.2.0" || imgB.EnvVarName != "B_VERSION" || imgB.EnvFilePath != filepath.Join(dirB, ".env") {
		t.Errorf("Image B metadata mismatch: %+v", imgB)
	}

	// Update versions
	imgA.NewVersion = "1.2.0"
	imgB.NewVersion = "2.3.0"

	if err := s.UpdateImages([]ContainerImage{*imgA, *imgB}); err != nil {
		t.Fatalf("UpdateImages failed: %v", err)
	}

	// Verify updated env contents
	envBytesA, err := os.ReadFile(filepath.Join(dirA, ".env"))
	if err != nil {
		t.Fatalf("Failed to read env-a: %v", err)
	}
	if string(envBytesA) != "A_VERSION=1.2.0\n" {
		t.Errorf("Expected A_VERSION=1.2.0\n, got %q", string(envBytesA))
	}

	envBytesB, err := os.ReadFile(filepath.Join(dirB, ".env"))
	if err != nil {
		t.Fatalf("Failed to read env-b: %v", err)
	}
	if string(envBytesB) != "B_VERSION=2.3.0\n" {
		t.Errorf("Expected B_VERSION=2.3.0\n, got %q", string(envBytesB))
	}
}
