package cli

import (
	"fmt"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// restartCmd represents the restart command
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart background server",
	Long: `Restart the model-router server running in background mode.

This stops the current server and starts a new one with the same
configuration.

Examples:
  # Restart server
  model-router restart

  # Restart with new config
  model-router restart --config /path/to/new/config.json`,
	RunE: runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)

	// Same flags as start
	restartCmd.Flags().IntP("port", "p", 0, "Server port (env: MODEL_ROUTER_PORT, default: 8080)")
	restartCmd.Flags().String("host", "", "Server host (env: MODEL_ROUTER_HOST, default: 0.0.0.0)")

	_ = viper.BindPFlag("server.port", restartCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("server.host", restartCmd.Flags().Lookup("host"))
}

func runRestart(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()
	dm := daemon.NewDaemonManager(cfg.GetEffectivePIDFile(), cfg.GetEffectiveLogFile())

	// Get config file path
	configPath := config.GetActiveConfigPath()

	if err := dm.Restart(configPath); err != nil {
		return fmt.Errorf("failed to restart daemon: %w", err)
	}

	fmt.Printf("Model Router restarted\n")
	fmt.Printf("Server: http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)

	return nil
}
