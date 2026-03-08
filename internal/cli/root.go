package cli

import (
	"fmt"
	"os"

	"github.com/gemone/model-router/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version is set at build time
	Version = "dev"
	// Commit is set at build time
	Commit = "none"
	// BuildDate is set at build time
	BuildDate = "unknown"
)

var (
	cfgFile string
	verbose bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "model-router",
	Short: "AI Model Routing Service",
	Long: `Model Router is an AI model routing service that provides
unified API access to multiple AI model providers.

It supports OpenAI, Claude, Azure, DeepSeek, Ollama, and other
OpenAI-compatible APIs through a single unified interface.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize config before running any command
		_, err := config.InitConfig(cfgFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ~/.config/model-router/config.json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Bind config flag to viper
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find config directory.
		configDir := config.GetDefaultConfigDir()

		viper.AddConfigPath(configDir)
		viper.AddConfigPath(".")
		viper.SetConfigType("json")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("MODEL_ROUTER")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

// GetVersion returns the version string
func GetVersion() string {
	return fmt.Sprintf("model-router %s (commit: %s, built: %s)", Version, Commit, BuildDate)
}
