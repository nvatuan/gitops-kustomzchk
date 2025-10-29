package runner

import "github.com/gh-nvat/gitops-kustomzchk/src/pkg/models"

type RunnerInterface interface {
	// Initialize the runner with necessary context and data
	Initialize() error

	// Build manifests for a specific environment,
	// Provided path will be appened with overlay names, then a kustomize build will be run on the resulting path
	BuildManifests(beforePath, afterPath string) (*models.BuildManifestResult, error)

	// Build manifests for a specific environment
	DiffManifests(*models.BuildManifestResult) (map[string]models.EnvironmentDiff, error)

	// Main routine to process the runner
	Process() error

	// Handling the export
	Output(data *models.ReportData) error
}
