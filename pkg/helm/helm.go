package helm

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// GetValuesFileByVersion fetches the default values.yaml for a specific chart version from a repository
func GetValuesFileByVersion(repoURL, chartName, version string) (string, error) {
	settings := cli.New()

	// Try pulling chart directly first (might be cached or available via URL)
	values, err := tryPullChart(chartName, version, repoURL, settings)
	if err == nil {
		return values, nil
	}

	// Chart not available - add repo and try again
	repoName, err := addRepoIfNotExists(repoURL, settings)
	if err != nil {
		return "", fmt.Errorf("failed to add repository: %w", err)
	}

	// Try pulling with repo/chart format
	chartRef := fmt.Sprintf("%s/%s", repoName, chartName)
	values, err = tryPullChart(chartRef, version, "", settings)
	if err != nil {
		return "", fmt.Errorf("failed to pull chart after adding repo: %w", err)
	}

	return values, nil
}

// tryPullChart attempts to pull and extract values from a chart
func tryPullChart(chartRef, version, repoURL string, settings *cli.EnvSettings) (string, error) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "hvu-chart-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Pull the chart
	err = pullChart(chartRef, version, repoURL, tmpDir, settings)
	if err != nil {
		return "", err
	}

	// Extract chart name from reference (handle "repo/chart" or just "chart")
	chartName := filepath.Base(chartRef)

	// Read values.yaml from extracted chart
	return readValuesFromChart(tmpDir, chartName)
}

// pullChart downloads and extracts a chart to the specified directory
func pullChart(chartRef, version, repoURL, destDir string, settings *cli.EnvSettings) error {
	actionConfig := &action.Configuration{}
	pullClient := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pullClient.Settings = settings
	pullClient.Version = version
	pullClient.DestDir = destDir
	pullClient.Untar = true

	if repoURL != "" {
		pullClient.RepoURL = repoURL
	}

	_, err := pullClient.Run(chartRef)
	return err
}

// readValuesFromChart reads the values.yaml file from an extracted chart directory
func readValuesFromChart(tmpDir, chartName string) (string, error) {
	chartPath := filepath.Join(tmpDir, chartName)
	showClient := action.NewShow(action.ShowValues)
	output, err := showClient.Run(chartPath)
	if err != nil {
		return "", fmt.Errorf("failed to read values from chart: %w", err)
	}
	return output, nil
}

// addRepoIfNotExists adds a Helm repository if it doesn't exist and returns the repo name
func addRepoIfNotExists(repoURL string, settings *cli.EnvSettings) (string, error) {
	// Check if repo already exists
	existingRepoName := findRepoByURL(repoURL, settings)
	if existingRepoName != "" {
		// Repo already exists, update its index
		err := updateRepoIndex(existingRepoName, repoURL, settings)
		if err != nil {
			return "", fmt.Errorf("failed to update existing repo: %w", err)
		}
		return existingRepoName, nil
	}

	// Repo doesn't exist - add it
	return addNewRepo(repoURL, settings)
}

// findRepoByURL checks if a repo with the given URL already exists
func findRepoByURL(repoURL string, settings *cli.EnvSettings) string {
	repos, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		return ""
	}

	for _, existing := range repos.Repositories {
		if existing.URL == repoURL {
			return existing.Name
		}
	}
	return ""
}

// updateRepoIndex updates the index for an existing repository
func updateRepoIndex(repoName, repoURL string, settings *cli.EnvSettings) error {
	providers := getter.All(settings)
	entry := &repo.Entry{
		Name: repoName,
		URL:  repoURL,
	}

	chartRepo, err := repo.NewChartRepository(entry, providers)
	if err != nil {
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download repository index: %w", err)
	}

	return nil
}

// addNewRepo adds a new repository to Helm and returns its name
func addNewRepo(repoURL string, settings *cli.EnvSettings) (string, error) {
	providers := getter.All(settings)

	// Load or create repo file
	repoFile := settings.RepositoryConfig
	repos, err := repo.LoadFile(repoFile)
	if err != nil {
		repos = repo.NewFile()
	}

	// Generate unique repo name based on URL hash to avoid conflicts
	hash := sha256.Sum256([]byte(repoURL))
	repoName := fmt.Sprintf("hvu-%x", hash[:8])

	// Create repo entry
	entry := &repo.Entry{
		Name: repoName,
		URL:  repoURL,
	}

	// Create and download index
	chartRepo, err := repo.NewChartRepository(entry, providers)
	if err != nil {
		return "", fmt.Errorf("failed to create chart repository: %w", err)
	}

	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return "", fmt.Errorf("failed to download repository index: %w", err)
	}

	// Add to repository file
	repos.Add(entry)
	if err := repos.WriteFile(repoFile, 0644); err != nil {
		return "", fmt.Errorf("failed to write repository file: %w", err)
	}

	return repoName, nil
}
