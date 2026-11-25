package config

import (
	"testing"

	"github.com/spf13/viper"
)

// isolate keeps Load from reading config files in the user's home directory.
func isolate(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Setenv("HOME", t.TempDir())
	t.Cleanup(viper.Reset)
}

func TestLoadAutoSelectMaxDefault(t *testing.T) {
	isolate(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoSelectMax != "major" {
		t.Errorf("expected default %q, got %q", "major", cfg.AutoSelectMax)
	}
}

func TestLoadAutoSelectMaxFromEnv(t *testing.T) {
	isolate(t)
	t.Setenv("DCUM_AUTOSELECT_MAX", " Minor ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AutoSelectMax != "minor" {
		t.Errorf("expected %q, got %q", "minor", cfg.AutoSelectMax)
	}
}

func TestLoadAutoSelectMaxInvalid(t *testing.T) {
	isolate(t)
	t.Setenv("DCUM_AUTOSELECT_MAX", "minro")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid autoselect_max")
	}
}
