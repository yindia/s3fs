package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var logLevel slog.Level
var cfgFile string
var (
	LogLevel string
	address  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "s3fs",
	Short: "A CLI application for managing s3fs server",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, display the help message
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Here you can define flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,

	// Add global flag for log level
	rootCmd.PersistentFlags().StringVar(&LogLevel, "log-level", "error", "Set the logging level")

	// Add global flag for address
	rootCmd.PersistentFlags().StringVar(&address, "address", "http://127.0.0.1:8080", "Set the server address")

	// Modify the PersistentPreRun function
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		InitLogger(LogLevel)
	}
}

// InitLogger initializes the global logger with the specified log level
func InitLogger(level string) {

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		fmt.Printf("Invalid log level: %s. Using 'info' as default.\n", level)
		logLevel = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)
}
