package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var (
	globalViper *viper.Viper
	globalCfg   *Config
)

// InitConfig initializes the configuration using Viper
// It loads config from file, environment variables, and sets defaults
func InitConfig(cfgFile string) (*Config, error) {
	// Use the global viper instance to ensure consistency with cli bindings
	v := viper.GetViper()

	// Set defaults
	setDefaults(v)

	// Setup config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		// Check for MODEL_ROUTER_CONFIG_PATH env var
		if envPath := os.Getenv("MODEL_ROUTER_CONFIG_PATH"); envPath != "" {
			v.SetConfigFile(envPath)
		} else {
			configDir := GetDefaultConfigDir()
			v.SetConfigName("config")
			v.SetConfigType("json")
			v.AddConfigPath(configDir)
			v.AddConfigPath(".")
		}
	}

	// Setup environment variables
	v.SetEnvPrefix("MODEL_ROUTER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific environment variables for nested config
	bindEnvVars(v)

	// Try to read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, create default config
			configPath := getConfigPath(cfgFile)
			if err := createDefaultConfigFile(configPath); err != nil {
				// If we can't create the config file, just use defaults
				fmt.Printf("Warning: could not create default config file: %v\n", err)
			} else {
				fmt.Printf("Created default config file: %s\n", configPath)
				// Re-read the config
				if err := v.ReadInConfig(); err != nil {
					fmt.Printf("Warning: could not read created config file: %v\n", err)
				}
			}
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal to struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Store globally
	globalViper = v
	globalCfg = &cfg

	return &cfg, nil
}

// setDefaults sets all default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 60)

	// Database defaults
	v.SetDefault("database.path", "")

	// Security defaults (empty, must be set by user)
	v.SetDefault("security.admin_token", "")
	v.SetDefault("security.jwt_secret", "")
	v.SetDefault("security.encryption_key", "")

	// CORS defaults
	v.SetDefault("cors.enabled", true)
	v.SetDefault("cors.allowed_origins", []string{
		"http://localhost:3000",
		"http://localhost:5173",
	})
	v.SetDefault("cors.enable_https", false)

	// Features defaults
	v.SetDefault("features.enable_stats", true)
	v.SetDefault("features.enable_fallback", true)
	v.SetDefault("features.max_retries", 3)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.buffer_size", 1000)

	// UI defaults
	v.SetDefault("ui.auto_open", false)

	// Daemon defaults
	v.SetDefault("daemon.pid_file", "")
	v.SetDefault("daemon.log_file", "")
}

// bindEnvVars binds environment variables to config keys
func bindEnvVars(v *viper.Viper) {
	// Server
	_ = v.BindEnv("server.host", "MODEL_ROUTER_HOST")
	_ = v.BindEnv("server.port", "MODEL_ROUTER_PORT")
	_ = v.BindEnv("server.read_timeout", "MODEL_ROUTER_READ_TIMEOUT")
	_ = v.BindEnv("server.write_timeout", "MODEL_ROUTER_WRITE_TIMEOUT")

	// Database
	_ = v.BindEnv("database.path", "MODEL_ROUTER_DB_PATH")

	// Security
	_ = v.BindEnv("security.admin_token", "MODEL_ROUTER_ADMIN_TOKEN")
	_ = v.BindEnv("security.jwt_secret", "MODEL_ROUTER_JWT_SECRET")
	_ = v.BindEnv("security.encryption_key", "MODEL_ROUTER_ENCRYPTION_KEY")

	// CORS
	_ = v.BindEnv("cors.enabled", "MODEL_ROUTER_ENABLE_CORS")
	_ = v.BindEnv("cors.allowed_origins", "MODEL_ROUTER_ALLOWED_ORIGINS")
	_ = v.BindEnv("cors.enable_https", "MODEL_ROUTER_ENABLE_HTTPS")

	// Features
	_ = v.BindEnv("features.enable_stats", "MODEL_ROUTER_ENABLE_STATS")
	_ = v.BindEnv("features.enable_fallback", "MODEL_ROUTER_ENABLE_FALLBACK")
	_ = v.BindEnv("features.max_retries", "MODEL_ROUTER_MAX_RETRIES")

	// Logging
	_ = v.BindEnv("logging.level", "MODEL_ROUTER_LOG_LEVEL")
	_ = v.BindEnv("logging.buffer_size", "MODEL_ROUTER_LOG_BUFFER_SIZE")

	// UI
	_ = v.BindEnv("ui.auto_open", "MODEL_ROUTER_UI_AUTO_OPEN")

	// Daemon
	_ = v.BindEnv("daemon.pid_file", "MODEL_ROUTER_PID_FILE")
	_ = v.BindEnv("daemon.log_file", "MODEL_ROUTER_LOG_FILE")
}

// getConfigPath returns the config file path to use
func getConfigPath(cfgFile string) string {
	if cfgFile != "" {
		return cfgFile
	}
	if envPath := os.Getenv("MODEL_ROUTER_CONFIG_PATH"); envPath != "" {
		return envPath
	}
	return GetDefaultConfigPath()
}

// createDefaultConfigFile creates a default config file at the specified path
func createDefaultConfigFile(path string) error {
	// Ensure directory exists
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	// Create default config
	cfg := DefaultConfig()

	// Write to file
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetViper returns the global Viper instance
func GetViper() *viper.Viper {
	return globalViper
}

// GetConfig returns the global Config instance
func GetConfig() *Config {
	if globalCfg == nil {
		// This shouldn't happen if InitConfig was called
		globalCfg = DefaultConfig()
	}
	return globalCfg
}

// SetConfigValue sets a configuration value at runtime
func SetConfigValue(key string, value interface{}) {
	if globalViper != nil {
		globalViper.Set(key, value)
	}
}

// Load is kept for backward compatibility
// It loads the configuration and returns it
func Load() *Config {
	if globalCfg != nil {
		return globalCfg
	}

	cfg, err := InitConfig("")
	if err != nil {
		fmt.Printf("Warning: error loading config: %v, using defaults\n", err)
		return DefaultConfig()
	}
	return cfg
}

// Get is kept for backward compatibility
func Get() *Config {
	if globalCfg == nil {
		return Load()
	}
	return globalCfg
}
