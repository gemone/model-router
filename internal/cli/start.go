package cli

import (
	"fmt"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startCmd represents the start command (daemon mode)
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start server in background (daemon mode)",
	Long: `Start the model-router server in background mode.

The server will run as a daemon process and continue running
after the current terminal session ends.

Examples:
  # Start server in background
  model-router start

  # Start with custom config
  model-router start --config /path/to/config.json

  # Start with custom port
  model-router start --port 9000`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)

	// Same flags as serve command
	startCmd.Flags().IntP("port", "p", 0, "Server port (env: MODEL_ROUTER_PORT, default: 8080)")
	startCmd.Flags().String("host", "", "Server host (env: MODEL_ROUTER_HOST, default: 0.0.0.0)")
	startCmd.Flags().String("db-path", "", "Database path (env: MODEL_ROUTER_DB_PATH)")
	startCmd.Flags().String("admin-token", "", "Admin authentication token (env: MODEL_ROUTER_ADMIN_TOKEN)")
	startCmd.Flags().String("jwt-secret", "", "JWT signing secret (env: MODEL_ROUTER_JWT_SECRET)")
	startCmd.Flags().String("encryption-key", "", "Data encryption key (env: MODEL_ROUTER_ENCRYPTION_KEY)")
	startCmd.Flags().String("log-level", "", "Log level: debug/info/warn/error (env: MODEL_ROUTER_LOG_LEVEL)")
	startCmd.Flags().String("pid-file", "", "PID file path (env: MODEL_ROUTER_PID_FILE)")
	startCmd.Flags().String("log-file", "", "Log file path (env: MODEL_ROUTER_LOG_FILE)")

	// Bind flags to viper
	_ = viper.BindPFlag("server.port", startCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("server.host", startCmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("database.path", startCmd.Flags().Lookup("db-path"))
	_ = viper.BindPFlag("security.admin_token", startCmd.Flags().Lookup("admin-token"))
	_ = viper.BindPFlag("security.jwt_secret", startCmd.Flags().Lookup("jwt-secret"))
	_ = viper.BindPFlag("security.encryption_key", startCmd.Flags().Lookup("encryption-key"))
	_ = viper.BindPFlag("logging.level", startCmd.Flags().Lookup("log-level"))
	_ = viper.BindPFlag("daemon.pid_file", startCmd.Flags().Lookup("pid-file"))
	_ = viper.BindPFlag("daemon.log_file", startCmd.Flags().Lookup("log-file"))
}

func runStart(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()

	// Create daemon manager
	dm := daemon.NewDaemonManager(cfg.GetEffectivePIDFile(), cfg.GetEffectiveLogFile())

	// Get config file path
	configPath := config.GetActiveConfigPath()

	// Start daemon
	if err := dm.Start(configPath); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("Model Router started in background\n")
	fmt.Printf("PID file: %s\n", cfg.GetEffectivePIDFile())
	fmt.Printf("Log file: %s\n", cfg.GetEffectiveLogFile())
	fmt.Printf("Server: http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)

	return nil
}
