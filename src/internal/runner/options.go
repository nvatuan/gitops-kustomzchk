package runner

import "github.com/gh-nvat/gitops-kustomzchk/src/pkg/pathbuilder"

type GitCheckoutStrategy string

const (
	GitCheckoutStrategySparse  GitCheckoutStrategy = "sparse"
	GitCheckoutStrategyShallow GitCheckoutStrategy = "shallow"
)

type Options struct {
	// Run mode
	RunMode string // "github" or "local"
	Debug   bool   // Debug mode

	// Common options
	PoliciesPath                  string
	TemplatesPath                 string
	OutputDir                     string
	EnableExportReport            bool
	EnableExportPerformanceReport bool
	FailOnOverlayNotFound         bool // Fail if overlay doesn't exist (default: false, skip gracefully)

	// === Legacy flags (v0.4 backward compatibility) ===
	Service      string   // Deprecated: use KustomizeBuildPath + KustomizeBuildValues
	Environments []string // Deprecated: use KustomizeBuildPath + KustomizeBuildValues

	// === New dynamic path flags (v0.5+) ===
	KustomizeBuildPath   string // Template path with $VARIABLES (e.g., "/path/$SERVICE/clusters/$CLUSTER/$ENV")
	KustomizeBuildValues string // Variable values: "KEY=v1,v2;KEY2=v3"

	// Computed internally from the new flags
	PathBuilder *pathbuilder.PathBuilder

	// GitHub mode options
	GhRepo              string
	GhPrNumber          int
	ManifestsPath       string              // Path to services directory (default: ./services)
	GitCheckoutStrategy GitCheckoutStrategy // Git checkout strategy: sparse (scoped) or shallow (all files)

	// Local mode options
	LcBeforeManifestsPath string
	LcAfterManifestsPath  string
}

// UseDynamicPaths returns true if new dynamic path flags are used
func (o *Options) UseDynamicPaths() bool {
	return o.KustomizeBuildPath != "" && o.KustomizeBuildValues != ""
}

// InitializePathBuilder creates a PathBuilder from the new flags
func (o *Options) InitializePathBuilder() error {
	if !o.UseDynamicPaths() {
		return nil
	}
	pb, err := pathbuilder.NewPathBuilder(o.KustomizeBuildPath, o.KustomizeBuildValues)
	if err != nil {
		return err
	}
	if err := pb.Validate(); err != nil {
		return err
	}
	o.PathBuilder = pb
	return nil
}
