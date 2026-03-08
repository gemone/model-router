package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/gemone/model-router/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage model-router configuration.

Subcommands:
  init   - Generate a default configuration file
  show   - Show current effective configuration
  check  - Validate configuration file`,
}

// configInitCmd represents the config init command
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate default configuration file",
	Long: `Generate a default configuration file at the default location
(~/.config/model-router/config.json on Linux/macOS).

If the file already exists, use --force to overwrite it.`,
	RunE: runConfigInit,
}

// configShowCmd represents the config show command
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current effective configuration",
	Long: `Display the current effective configuration.

This shows the merged configuration from:
- Default values
- Configuration file
- Environment variables
- Command-line flags`,
	RunE: runConfigShow,
}

// configCheckCmd represents the config check command
var configCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate configuration file",
	Long:  `Validate the configuration file for syntax errors and invalid values.`,
	RunE: runConfigCheck,
}

var (
	configForce    bool
	configOutput   string
	configShowJSON bool
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configCheckCmd)

	// config init flags
	configInitCmd.Flags().BoolVarP(&configForce, "force", "f", false, "Overwrite existing config file")
	configInitCmd.Flags().StringVarP(&configOutput, "output", "o", "", "Output file path (default: ~/.config/model-router/config.json)")

	// config show flags
	configShowCmd.Flags().BoolVar(&configShowJSON, "json", false, "Output in JSON format")
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	// Determine output path
	outputPath := configOutput
	if outputPath == "" {
		outputPath = config.GetDefaultConfigPath()
	}

	// Check if file exists
	if _, err := os.Stat(outputPath); err == nil && !configForce {
		return fmt.Errorf("config file already exists at %s, use --force to overwrite", outputPath)
	}

	// Create default config
	cfg := config.DefaultConfig()

	// Write to file
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	if err := config.EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Created default config file: %s\n", outputPath)
	fmt.Println("\nYou can now edit this file to customize your configuration.")
	fmt.Println("Required settings:")
	fmt.Println("  - security.encryption_key: Generate with 'openssl rand -base64 32'")
	fmt.Println("\nOptional but recommended:")
	fmt.Println("  - security.admin_token: Set to secure admin endpoints")

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	// Get current config
	cfg := config.GetConfig()

	if configShowJSON {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Pretty print
	fmt.Println("Model Router Configuration")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())
	fmt.Println()

	fmt.Println("Server:")
	fmt.Printf("  Host: %s\n", cfg.Server.Host)
	fmt.Printf("  Port: %d\n", cfg.Server.Port)
	fmt.Printf("  Read Timeout: %ds\n", cfg.Server.ReadTimeout)
	fmt.Printf("  Write Timeout: %ds\n", cfg.Server.WriteTimeout)
	fmt.Println()

	fmt.Println("Database:")
	fmt.Printf("  Path: %s\n", cfg.GetEffectiveDBPath())
	fmt.Println()

	fmt.Println("Security:")
	fmt.Printf("  Admin Token: %s\n", maskSecret(cfg.Security.AdminToken))
	fmt.Printf("  JWT Secret: %s\n", maskSecret(cfg.Security.JWTSecret))
	fmt.Printf("  Encryption Key: %s\n", maskSecret(cfg.Security.EncryptionKey))
	fmt.Println()

	fmt.Println("CORS:")
	fmt.Printf("  Enabled: %v\n", cfg.CORS.Enabled)
	fmt.Printf("  Allowed Origins: %s\n", strings.Join(cfg.CORS.AllowedOrigins, ", "))
	fmt.Printf("  Enable HTTPS: %v\n", cfg.CORS.EnableHTTPS)
	fmt.Println()

	fmt.Println("Features:")
	fmt.Printf("  Enable Stats: %v\n", cfg.Features.EnableStats)
	fmt.Printf("  Enable Fallback: %v\n", cfg.Features.EnableFallback)
	fmt.Printf("  Max Retries: %d\n", cfg.Features.MaxRetries)
	fmt.Println()

	fmt.Println("Logging:")
	fmt.Printf("  Level: %s\n", cfg.Logging.Level)
	fmt.Printf("  Buffer Size: %d\n", cfg.Logging.BufferSize)
	fmt.Println()

	fmt.Println("UI:")
	fmt.Printf("  Auto Open: %v\n", cfg.UI.AutoOpen)
	fmt.Println()

	fmt.Println("Daemon:")
	fmt.Printf("  PID File: %s\n", cfg.GetEffectivePIDFile())
	fmt.Printf("  Log File: %s\n", cfg.GetEffectiveLogFile())

	return nil
}

func runConfigCheck(cmd *cobra.Command, args []string) error {
	// Get current config
	cfg := config.GetConfig()

	errors := []string{}
	warnings := []string{}

	// Check required fields
	if cfg.Security.EncryptionKey == "" {
		errors = append(errors, "security.encryption_key is required")
	}

	// Check recommended fields
	if cfg.Security.AdminToken == "" {
		warnings = append(warnings, "security.admin_token is not set (admin endpoints will be unauthenticated)")
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid port: %d", cfg.Server.Port))
	}

	if cfg.Server.ReadTimeout < 1 {
		warnings = append(warnings, "server.read_timeout is very low")
	}

	if cfg.Server.WriteTimeout < 1 {
		warnings = append(warnings, "server.write_timeout is very low")
	}

	// Print results
	fmt.Println("Configuration Check")
	fmt.Println(strings.Repeat("=", 40))
	fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())
	fmt.Println()

	if len(errors) > 0 {
		fmt.Println("Errors:")
		for _, e := range errors {
			fmt.Printf("  ✗ %s\n", e)
		}
		fmt.Println()
	}

	if len(warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
		fmt.Println()
	}

	if len(errors) == 0 && len(warnings) == 0 {
		fmt.Println("✓ Configuration is valid")
		return nil
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration has %d error(s)", len(errors))
	}

	fmt.Println("Configuration has warnings but is usable")
	return nil
}

// maskSecret masks a secret string for display
func maskSecret(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}
