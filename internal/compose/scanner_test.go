package compose

import (
	"os"
	"path/filepath"
	"strings"
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

func TestScanner_Immich(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dcum-immich-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	composeContent := `name: immich

services:
  immich-server:
    container_name: immich
    image: ghcr.io/immich-app/immich-server:${IMMICH_VERSION:-release}
    extends:
      file: hwaccel.transcoding.yml
      service: quicksync # set to one of [nvenc, quicksync, rkmpp, vaapi, vaapi-wsl] for accelerated transcoding
    volumes:
      - ${UPLOAD_LOCATION}:/usr/src/app/upload
      - /etc/localtime:/etc/localtime:ro
    env_file:
      - .env
    ports:
      - 2283:2283
    depends_on:
      - redis
      - database
    restart: always
    labels:
      - 'traefik.enable=true'
      - 'wud.tag.include=^v\d+\.\d+\.\d+$$'
      - 'wud.link.template=https://github.com/immich-app/immich/releases/tag/v$${major}.$${minor}.$${patch}'

  immich-machine-learning:
    container_name: immich-machine-learning
    image: ghcr.io/immich-app/immich-machine-learning:${IMMICH_VERSION:-release}
    volumes:
      - model-cache:/cache
    env_file:
      - .env
    restart: always
    labels:
      - 'wud.tag.include=^v\d+\.\d+\.\d+$$'
      - 'wud.link.template=https://github.com/immich-app/immich/releases/tag/v$${major}.$${minor}.$${patch}'

  redis:
    container_name: immich-redis
    image: docker.io/valkey/valkey:9@sha256:4963247afc4cd33c7d3b2d2816b9f7f8eeebab148d29056c2ca4d7cbc966f2d9
    healthcheck:
      test: redis-cli ping || exit 1
    restart: always

  database:
    container_name: immich-postgres
    image: ghcr.io/immich-app/postgres:14-vectorchord0.4.3-pgvectors0.2.0@sha256:bcf63357191b76a916ae5eb93464d65c07511da41e3bf7a8416db519b40b1c23
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_USER: ${DB_USERNAME}
      POSTGRES_DB: ${DB_DATABASE_NAME}
      POSTGRES_INITDB_ARGS: '--data-checksums'
    volumes:
      - ${DB_DATA_LOCATION}:/var/lib/postgresql/data
    shm_size: 128mb
    restart: always
    healthcheck:
      disable: false

volumes:
  model-cache:
`

	envContent := `# The location where your uploaded files are stored
UPLOAD_LOCATION=/data/immich/library
# The location where your database files are stored
DB_DATA_LOCATION=/data/immich/postgres

# To set a timezone, uncomment the next line and change Etc/UTC to a TZ identifier from this list: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones#List
TZ=Europe/Warsaw

# The Immich version to use. You can pin this to a specific version like "v1.71.0"
IMMICH_VERSION=v3.2.4
`

	if err := os.WriteFile(filepath.Join(tempDir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write env: %v", err)
	}

	s := NewScanner([]string{})
	images, err := s.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	t.Logf("Found %d images: %+v", len(images), images)

	for i := range images {
		if images[i].ServiceName == "immich-server" {
			images[i].NewVersion = "v3.3.0"
		}
	}

	if err := s.UpdateImages(images); err != nil {
		t.Fatalf("UpdateImages failed: %v", err)
	}

	updatedEnvBytes, err := os.ReadFile(filepath.Join(tempDir, ".env"))
	if err != nil {
		t.Fatalf("Failed to read updated env: %v", err)
	}

	t.Logf("Updated .env:\n%s", string(updatedEnvBytes))

	if !strings.Contains(string(updatedEnvBytes), "IMMICH_VERSION=v3.3.0") {
		t.Errorf("Expected IMMICH_VERSION=v3.3.0 in .env, but got:\n%s", string(updatedEnvBytes))
	}
}

func TestScanner_Immich_WithExtendsFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dcum-immich-hwaccel-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	composeContent := `name: immich

services:
  immich-server:
    container_name: immich
    image: ghcr.io/immich-app/immich-server:${IMMICH_VERSION:-release}
    extends:
      file: hwaccel.transcoding.yml
      service: quicksync
    volumes:
      - ${UPLOAD_LOCATION}:/usr/src/app/upload
    env_file:
      - .env

  database:
    image: postgres:14
    volumes:
      - ${DB_DATA_LOCATION}:/var/lib/postgresql/data
`

	envContent := `UPLOAD_LOCATION=/data/immich/library
DB_DATA_LOCATION=/data/immich/postgres
IMMICH_VERSION=v3.2.4
`

	hwaccelContent := `services:
  quicksync:
    devices:
      - /dev/dri:/dev/dri
`

	if err := os.WriteFile(filepath.Join(tempDir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("Failed to write compose: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "hwaccel.transcoding.yml"), []byte(hwaccelContent), 0644); err != nil {
		t.Fatalf("Failed to write hwaccel: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write env: %v", err)
	}

	s := NewScanner([]string{})
	images, err := s.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for i := range images {
		if images[i].ServiceName == "immich-server" {
			images[i].NewVersion = "v3.3.0"
		}
	}

	if err := s.UpdateImages(images); err != nil {
		t.Fatalf("UpdateImages failed: %v", err)
	}

	updatedEnvBytes, err := os.ReadFile(filepath.Join(tempDir, ".env"))
	if err != nil {
		t.Fatalf("Failed to read updated env: %v", err)
	}

	if !strings.Contains(string(updatedEnvBytes), "IMMICH_VERSION=v3.3.0") {
		t.Errorf("Expected IMMICH_VERSION=v3.3.0 in .env, but got:\n%s", string(updatedEnvBytes))
	}
}


