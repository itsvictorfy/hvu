package helm

import (
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

// GetValuesFileByVersion fetches the default values.yaml for a specific chart version from a repository
func GetValuesFileByVersion(repoURL, chartName, version string) (string, error) {
	settings := cli.New()
	client := action.NewShow(action.ShowValues)

	client.ChartPathOptions.RepoURL = repoURL
	client.ChartPathOptions.Version = version

	cp, err := client.ChartPathOptions.LocateChart(chartName, settings)
	if err != nil {
		return "", fmt.Errorf("failed to locate chart %s version %s: %w", chartName, version, err)
	}

	output, err := client.Run(cp)
	if err != nil {
		return "", fmt.Errorf("failed to read values for chart %s version %s: %w", chartName, version, err)
	}

	return output, nil
}
