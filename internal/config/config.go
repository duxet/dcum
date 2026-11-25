package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	// ExcludePatterns is a list of glob patterns for files/directories to exclude from scanning
	ExcludePatterns []string `mapstructure:"exclude_patterns"`

	// AutoSelectMax is the highest update level (patch, minor or major) that is
	// selected automatically for all services. The dcum.autoselect.max label
	// overrides it per service.
	AutoSelectMax string `mapstructure:"autoselect_max"`

	// UI configuration
	UI struct {
		ShowContainer bool `mapstructure:"show_container"`
	} `mapstructure:"ui"`
}

// Load loads the configuration from file and environment variables.
func Load() (*Config, error) {
	viper.SetConfigName("dcum")
	viper.SetConfigType("yaml")

	// Look for config in these locations (in order):
	// 1. Current directory
	viper.AddConfigPath(".")

	// 2. Home directory
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(home, ".config", "dcum"))
		viper.AddConfigPath(home)
	}

	// 3. System-wide config
	viper.AddConfigPath("/etc/dcum")

	// Set defaults
	viper.SetDefault("exclude_patterns", []string{
		"**/node_modules/**",
		"**/.git/**",
		"**/vendor/**",
	})
	viper.SetDefault("autoselect_max", "major")
	viper.SetDefault("ui.show_container", false)

	// Allow environment variables with DCUM_ prefix
	viper.SetEnvPrefix("DCUM")
	viper.AutomaticEnv()

	// Read config file (it's OK if it doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found; use defaults
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	cfg.AutoSelectMax = strings.ToLower(strings.TrimSpace(cfg.AutoSelectMax))
	switch cfg.AutoSelectMax {
	case "patch", "minor", "major":
	default:
		return nil, fmt.Errorf("invalid autoselect_max %q: must be patch, minor or major", cfg.AutoSelectMax)
	}

	return &cfg, nil
}
