package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

// GetHomeDir returns the user's home directory with fallback
func GetHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return os.Getenv("HOME")
	}
	return homeDir
}

// GetActiveConfigPath returns the currently active config file path
// Returns the config file used by Viper, or the default path if not set
func GetActiveConfigPath() string {
	if path := viper.ConfigFileUsed(); path != "" {
		return path
	}
	return GetDefaultConfigPath()
}

// GetDefaultConfigDir returns the OS-appropriate config directory for model-router
// Linux: ~/.config/model-router/
// macOS: ~/Library/Application Support/model-router/
// Windows: %APPDATA%\model-router\
func GetDefaultConfigDir() string {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		baseDir = filepath.Join(GetHomeDir(), "Library", "Application Support")
	default: // Linux and other Unix-like systems
		baseDir = os.Getenv("XDG_CONFIG_HOME")
		if baseDir == "" {
			baseDir = filepath.Join(GetHomeDir(), ".config")
		}
	}

	return filepath.Join(baseDir, "model-router")
}

// GetDefaultDataDir returns the OS-appropriate data directory for model-router
// Linux: ~/.local/share/model-router/
// macOS: ~/Library/Application Support/model-router/
// Windows: %APPDATA%\model-router\
func GetDefaultDataDir() string {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("APPDATA")
		if baseDir == "" {
			baseDir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
	case "darwin":
		baseDir = filepath.Join(GetHomeDir(), "Library", "Application Support")
	default: // Linux and other Unix-like systems
		baseDir = os.Getenv("XDG_DATA_HOME")
		if baseDir == "" {
			baseDir = filepath.Join(GetHomeDir(), ".local", "share")
		}
	}

	return filepath.Join(baseDir, "model-router")
}

// GetDefaultConfigPath returns the default config file path
func GetDefaultConfigPath() string {
	return filepath.Join(GetDefaultConfigDir(), "config.json")
}

// GetDefaultDBPath returns the default database file path
func GetDefaultDBPath() string {
	return filepath.Join(GetDefaultDataDir(), "data.db")
}

// GetDefaultPIDPath returns the default PID file path for daemon mode
func GetDefaultPIDPath() string {
	return filepath.Join(GetDefaultDataDir(), "model-router.pid")
}

// GetDefaultLogPath returns the default log file path for daemon mode
func GetDefaultLogPath() string {
	return filepath.Join(GetDefaultDataDir(), "model-router.log")
}

// GetLegacyDataDir returns the old data directory path for migration
// This is ~/.model-router/ which was used in previous versions
func GetLegacyDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	return filepath.Join(homeDir, ".model-router")
}

// GetLegacyDBPath returns the old database path for migration
func GetLegacyDBPath() string {
	return filepath.Join(GetLegacyDataDir(), "data.db")
}

// EnsureDir ensures a directory exists, creating it if necessary
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// EnsureConfigDir ensures the config directory exists
func EnsureConfigDir() error {
	return EnsureDir(GetDefaultConfigDir())
}

// EnsureDataDir ensures the data directory exists
func EnsureDataDir() error {
	return EnsureDir(GetDefaultDataDir())
}
