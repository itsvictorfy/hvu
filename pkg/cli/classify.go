package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/itsvictorfy/hvu/pkg/service"
	"github.com/itsvictorfy/hvu/pkg/values"
)

func ClassifyCmd() *cobra.Command {
	var (
		chart      string
		repository string
		version    string
		valuesFile string
	)

	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Classify values as customizations or defaults",
		Long: `Analyze a values file and classify each key as either a user
customization or a copied default from the chart.

This helps understand which values need to be preserved during an upgrade
and which can be replaced with new defaults.

Classification categories:
  CUSTOMIZED     - Value differs from chart default (intentional change)
  COPIED_DEFAULT - Value matches chart default (can be updated)
  UNKNOWN        - Not in chart defaults (may be obsolete or custom)

Examples:
  # Classify values against chart version
  hvu classify --chart postgresql \
    --repo https://charts.bitnami.com/bitnami \
    --version 12.1.0 --values ./my-values.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			slog.Info("classifying values",
				"chart", chart,
				"repository", repository,
				"version", version,
				"valuesFile", valuesFile,
			)

			output, err := service.Classify(&service.ClassifyInput{
				Chart:      chart,
				Repository: repository,
				Version:    version,
				ValuesFile: valuesFile,
			})
			if err != nil {
				return err
			}

			printClassifyResults(output)
			return nil
		},
	}

	// Chart identification
	cmd.Flags().StringVar(&chart, "chart", "", "chart name")
	cmd.Flags().StringVar(&repository, "repo", "", "chart repository URL")
	cmd.Flags().StringVar(&version, "version", "", "chart version to compare against")

	// Values input
	cmd.Flags().StringVarP(&valuesFile, "values", "f", "", "values file to classify")

	// Required flags
	_ = cmd.MarkFlagRequired("chart")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("version")
	_ = cmd.MarkFlagRequired("values")

	return cmd
}

func printClassifyResults(output *service.ClassifyOutput) {
	result := output.Result

	fmt.Println("Classification Results")
	fmt.Println("======================")
	fmt.Println()

	// Summary
	fmt.Printf("Summary:\n")
	fmt.Printf("  CUSTOMIZED:     %d keys (user modifications)\n", result.Customized)
	fmt.Printf("  COPIED_DEFAULT: %d keys (match chart defaults)\n", result.CopiedDefault)
	fmt.Printf("  UNKNOWN:        %d keys (not in chart defaults)\n", result.Unknown)
	fmt.Printf("  Total:          %d keys\n", result.Total)
	fmt.Println()

	// Detailed output
	if result.Customized > 0 {
		fmt.Println("CUSTOMIZED (user modifications to preserve):")
		fmt.Println("--------------------------------------------")
		for _, entry := range result.Entries {
			if entry.Classification == values.Customized {
				fmt.Printf("  %s\n", values.PathToDisplayFormat(entry.Path))
				fmt.Printf("    user:    %v\n", entry.UserValue)
				fmt.Printf("    default: %v\n", entry.DefaultValue)
			}
		}
		fmt.Println()
	}

	if result.Unknown > 0 {
		fmt.Println("UNKNOWN (not in chart defaults - may be obsolete):")
		fmt.Println("--------------------------------------------------")
		for _, entry := range result.Entries {
			if entry.Classification == values.Unknown {
				fmt.Printf("  %s: %v\n", values.PathToDisplayFormat(entry.Path), entry.UserValue)
			}
		}
		fmt.Println()
	}
}
