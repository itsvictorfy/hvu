package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "hvu",
	Short: "Helm values file upgrade tool",
	Long: `hvu (Helm Values Upgrade) helps safely upgrade Helm chart values.yaml files
when moving to newer chart versions.

It analyzes differences between chart versions, classifies user customizations vs
copied defaults, and generates an upgraded values file preserving your changes.

Examples:
  # Upgrade values file to new chart version
  hvu upgrade --chart postgresql \
    --repo https://charts.bitnami.com/bitnami \
    --from 12.1.0 --to 16.0.0 --values ./my-values.yaml

  # Classify values in a file (show customizations vs defaults)
  hvu classify --chart postgresql \
    --repo https://charts.bitnami.com/bitnami \
    --version 12.1.0 --values ./my-values.yaml`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("output", "o", "./upgrade-output",
		"output directory for generated files")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false,
		"suppress non-essential output")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false,
		"enable verbose logging")

	// Bind flags to viper
	_ = viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))
	_ = viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Add subcommands
	rootCmd.AddCommand(UpgradeCmd())
	rootCmd.AddCommand(ClassifyCmd())
	rootCmd.AddCommand(VersionCmd())
}

func setupLogging() {
	level := slog.LevelWarn
	if viper.GetBool("verbose") {
		level = slog.LevelDebug
	} else if viper.GetBool("quiet") {
		level = slog.LevelError
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})
	slog.SetDefault(slog.New(handler))
}
