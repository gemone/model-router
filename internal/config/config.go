package config

// Config is the main configuration structure for model-router
type Config struct {
	Server   ServerConfig   `mapstructure:"server" json:"server"`
	Database DatabaseConfig `mapstructure:"database" json:"database"`
	Security SecurityConfig `mapstructure:"security" json:"security"`
	CORS     CORSConfig     `mapstructure:"cors" json:"cors"`
	Features FeaturesConfig `mapstructure:"features" json:"features"`
	Logging  LoggingConfig  `mapstructure:"logging" json:"logging"`
	UI       UIConfig       `mapstructure:"ui" json:"ui"`
	Daemon   DaemonConfig   `mapstructure:"daemon" json:"daemon"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host         string `mapstructure:"host" json:"host"`
	Port         int    `mapstructure:"port" json:"port"`
	ReadTimeout  int    `mapstructure:"read_timeout" json:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout" json:"write_timeout"`
}

// DatabaseConfig contains database-related configuration
type DatabaseConfig struct {
	Path string `mapstructure:"path" json:"path"`
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	AdminToken    string `mapstructure:"admin_token" json:"admin_token"`
	JWTSecret     string `mapstructure:"jwt_secret" json:"jwt_secret"`
	EncryptionKey string `mapstructure:"encryption_key" json:"encryption_key"`
}

// CORSConfig contains CORS-related configuration
type CORSConfig struct {
	Enabled        bool     `mapstructure:"enabled" json:"enabled"`
	AllowedOrigins []string `mapstructure:"allowed_origins" json:"allowed_origins"`
	EnableHTTPS    bool     `mapstructure:"enable_https" json:"enable_https"`
}

// FeaturesConfig contains feature flags
type FeaturesConfig struct {
	EnableStats    bool `mapstructure:"enable_stats" json:"enable_stats"`
	EnableFallback bool `mapstructure:"enable_fallback" json:"enable_fallback"`
	MaxRetries     int  `mapstructure:"max_retries" json:"max_retries"`
}

// LoggingConfig contains logging-related configuration
type LoggingConfig struct {
	Level      string `mapstructure:"level" json:"level"`
	BufferSize int    `mapstructure:"buffer_size" json:"buffer_size"`
}

// UIConfig contains UI-related configuration
type UIConfig struct {
	AutoOpen bool `mapstructure:"auto_open" json:"auto_open"`
}

// DaemonConfig contains daemon-related configuration
type DaemonConfig struct {
	PIDFile string `mapstructure:"pid_file" json:"pid_file"`
	LogFile string `mapstructure:"log_file" json:"log_file"`
}

// GetAllowedOriginsString returns allowed origins as a comma-separated string
// for compatibility with existing code
func (c *CORSConfig) GetAllowedOriginsString() string {
	if len(c.AllowedOrigins) == 0 {
		return ""
	}
	result := c.AllowedOrigins[0]
	for i := 1; i < len(c.AllowedOrigins); i++ {
		result += "," + c.AllowedOrigins[i]
	}
	return result
}

// DefaultConfig returns a Config with all default values set
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30,
			WriteTimeout: 60,
		},
		Database: DatabaseConfig{
			Path: "",
		},
		Security: SecurityConfig{
			AdminToken:    "",
			JWTSecret:     "",
			EncryptionKey: "",
		},
		CORS: CORSConfig{
			Enabled: true,
			AllowedOrigins: []string{
				"http://localhost:3000",
				"http://localhost:5173",
			},
			EnableHTTPS: false,
		},
		Features: FeaturesConfig{
			EnableStats:    true,
			EnableFallback: true,
			MaxRetries:     3,
		},
		Logging: LoggingConfig{
			Level:      "info",
			BufferSize: 1000,
		},
		UI: UIConfig{
			AutoOpen: false,
		},
		Daemon: DaemonConfig{
			PIDFile: "",
			LogFile: "",
		},
	}
}

// GetEffectiveDBPath returns the effective database path
// If not set in config, returns the default path
func (c *Config) GetEffectiveDBPath() string {
	if c.Database.Path != "" {
		return c.Database.Path
	}
	return GetDefaultDBPath()
}

// GetEffectivePIDFile returns the effective PID file path
func (c *Config) GetEffectivePIDFile() string {
	if c.Daemon.PIDFile != "" {
		return c.Daemon.PIDFile
	}
	return GetDefaultPIDPath()
}

// GetEffectiveLogFile returns the effective log file path
func (c *Config) GetEffectiveLogFile() string {
	if c.Daemon.LogFile != "" {
		return c.Daemon.LogFile
	}
	return GetDefaultLogPath()
}

// ========== Backward Compatibility Accessors ==========
// These methods provide backward compatibility with the old flat Config struct

// Server fields
func (c *Config) GetPort() int            { return c.Server.Port }
func (c *Config) GetHost() string         { return c.Server.Host }
func (c *Config) GetReadTimeout() int     { return c.Server.ReadTimeout }
func (c *Config) GetWriteTimeout() int    { return c.Server.WriteTimeout }

// Database fields
func (c *Config) GetDBPath() string { return c.Database.Path }

// Security fields
func (c *Config) GetAdminToken() string    { return c.Security.AdminToken }
func (c *Config) GetJWTSecret() string     { return c.Security.JWTSecret }
func (c *Config) GetEncryptionKey() string { return c.Security.EncryptionKey }

// CORS fields
func (c *Config) GetEnableCORS() bool         { return c.CORS.Enabled }
func (c *Config) GetAllowedOrigins() string   { return c.CORS.GetAllowedOriginsString() }
func (c *Config) GetEnableHTTPS() bool        { return c.CORS.EnableHTTPS }

// Features fields
func (c *Config) GetEnableStats() bool    { return c.Features.EnableStats }
func (c *Config) GetEnableFallback() bool { return c.Features.EnableFallback }
func (c *Config) GetMaxRetries() int      { return c.Features.MaxRetries }

// Logging fields
func (c *Config) GetLogLevel() string      { return c.Logging.Level }
func (c *Config) GetLogBufferSize() int    { return c.Logging.BufferSize }

// UI fields
func (c *Config) GetUIAutoOpen() bool { return c.UI.AutoOpen }
