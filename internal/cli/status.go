package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/gemone/model-router/internal/config"
	"github.com/gemone/model-router/internal/daemon"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server status",
	Long: `Show the status of the model-router server.

Displays whether the server is running, its PID, uptime,
and other useful information.`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg := config.GetConfig()
	dm := daemon.NewDaemonManager(cfg.GetEffectivePIDFile(), cfg.GetEffectiveLogFile())

	status, err := dm.Status()
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Println("Model Router Status")
	fmt.Println(strings.Repeat("=", 40))

	if status.Running {
		fmt.Printf("Status:   Running\n")
		fmt.Printf("PID:      %d\n", status.PID)
		if status.Uptime > 0 {
			fmt.Printf("Uptime:   %s\n", formatDuration(status.Uptime))
		}
		fmt.Printf("Port:     %d\n", cfg.Server.Port)
		fmt.Printf("Host:     %s\n", cfg.Server.Host)
		fmt.Printf("Config:   %s\n", viper.ConfigFileUsed())
		fmt.Printf("PID File: %s\n", cfg.GetEffectivePIDFile())
		fmt.Printf("Log File: %s\n", cfg.GetEffectiveLogFile())
	} else {
		fmt.Printf("Status:   Stopped\n")
		fmt.Printf("Config:   %s\n", viper.ConfigFileUsed())
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm %.0fs", d.Minutes(), d.Seconds()-float64(int(d.Minutes())*60))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - hours*60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
