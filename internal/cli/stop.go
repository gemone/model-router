package cli

import (
	"fmt"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/daemon"
	"github.com/spf13/cobra"
)

var stopForce bool

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop background server",
	Long: `Stop the model-router server running in background mode.

By default, sends SIGTERM for graceful shutdown.
Use --force to send SIGKILL if graceful shutdown fails.`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&stopForce, "force", "f", false, "Force kill if graceful shutdown fails")
}

func runStop(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()
	dm := daemon.NewDaemonManager(cfg.GetEffectivePIDFile(), cfg.GetEffectiveLogFile())

	if err := dm.Stop(stopForce); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	fmt.Println("Model Router stopped")
	return nil
}
