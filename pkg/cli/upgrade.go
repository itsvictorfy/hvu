package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/itsvictorfy/hvu/pkg/service"
)

func UpgradeCmd() *cobra.Command {
	var (
		chart       string
		repository  string
		fromVersion string
		toVersion   string
		valuesFile  string
		outputDir   string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade a values file to a new chart version",
		Long: `Upgrade a Helm values file from one chart version to another.

This command:
1. Fetches default values for both source and target chart versions
2. Classifies your values as customizations vs copied defaults
3. Generates an upgraded values file preserving your customizations

Examples:
  # Basic upgrade
  hvu upgrade --chart postgresql \
    --repo https://charts.bitnami.com/bitnami \
    --from 12.1.0 --to 16.0.0 --values ./my-values.yaml

  # Specify output directory
  hvu upgrade --chart postgresql \
    --repo https://charts.bitnami.com/bitnami \
    --from 12.1.0 --to 16.0.0 --values ./my-values.yaml \
    --output ./upgraded

  # Dry run (preview without writing files)
  hvu upgrade --chart postgresql \
    --repo https://charts.bitnami.com/bitnami \
    --from 12.1.0 --to 16.0.0 --values ./my-values.yaml --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set default output directory
			if outputDir == "" {
				outputDir = viper.GetString("output")
				if outputDir == "" {
					outputDir = "."
				}
			}

			slog.Info("upgrading values file",
				"chart", chart,
				"repository", repository,
				"fromVersion", fromVersion,
				"toVersion", toVersion,
				"valuesFile", valuesFile,
				"outputDir", outputDir,
				"dryRun", dryRun,
			)

			output, err := service.Upgrade(&service.UpgradeInput{
				Chart:       chart,
				Repository:  repository,
				FromVersion: fromVersion,
				ToVersion:   toVersion,
				ValuesFile:  valuesFile,
				OutputDir:   outputDir,
				DryRun:      dryRun,
			})
			if err != nil {
				return err
			}

			printUpgradeResults(output, dryRun)
			return nil
		},
	}

	cmd.Flags().StringVar(&chart, "chart", "", "chart name")
	cmd.Flags().StringVar(&repository, "repo", "", "chart repository URL")

	cmd.Flags().StringVar(&fromVersion, "from", "", "source chart version")
	cmd.Flags().StringVar(&toVersion, "to", "", "target chart version")

	cmd.Flags().StringVarP(&valuesFile, "values", "f", "", "path to current values file")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "output directory (default: current directory)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without writing files")

	_ = cmd.MarkFlagRequired("chart")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.MarkFlagRequired("values")

	return cmd
}

func printUpgradeResults(output *service.UpgradeOutput, dryRun bool) {
	classification := output.Classification

	if dryRun {
		fmt.Println()
		fmt.Println("=== DRY RUN - Upgraded values.yaml ===")
		fmt.Println(output.UpgradedYAML)
		fmt.Println("=== END DRY RUN ===")
		return
	}

	fmt.Println()
	fmt.Printf("Upgrade complete!\n")
	fmt.Printf("  Output: %s\n", output.OutputPath)
	fmt.Println()
	fmt.Printf("Summary:\n")
	fmt.Printf("  %d customizations preserved\n", classification.Customized)
	fmt.Printf("  %d defaults updated to new version\n", classification.CopiedDefault)
	if classification.Unknown > 0 {
		fmt.Printf("  %d unknown keys kept (review recommended)\n", classification.Unknown)
	}
}
