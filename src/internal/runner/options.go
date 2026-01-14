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
	// For GitHub mode: single path template (before/after determined by git refs)
	KustomizeBuildPath   string // Template path with $VARIABLES (e.g., "services/$SERVICE/clusters/$CLUSTER/$ENV")
	KustomizeBuildValues string // Variable values: "KEY=v1,v2;KEY2=v3"

	// Computed internally from the new flags
	PathBuilder       *pathbuilder.PathBuilder
	BeforePathBuilder *pathbuilder.PathBuilder // For local mode with separate before path
	AfterPathBuilder  *pathbuilder.PathBuilder // For local mode with separate after path

	// GitHub mode options
	GhRepo              string
	GhPrNumber          int
	ManifestsPath       string              // Path to services directory (default: ./services)
	GitCheckoutStrategy GitCheckoutStrategy // Git checkout strategy: sparse (scoped) or shallow (all files)

	// Local mode options (legacy)
	LcBeforeManifestsPath string
	LcAfterManifestsPath  string

	// Local mode options (v0.5+ dynamic paths)
	LcBeforeKustomizeBuildPath string // Template for before path (e.g., "/path/before/services/$SERVICE/$ENV")
	LcAfterKustomizeBuildPath  string // Template for after path (e.g., "/path/after/services/$SERVICE/$ENV")
}

// UseDynamicPaths returns true if new dynamic path flags are used (GitHub mode or shared local mode)
func (o *Options) UseDynamicPaths() bool {
	return o.KustomizeBuildPath != "" && o.KustomizeBuildValues != ""
}

// UseLocalDynamicPaths returns true if local mode with separate before/after paths is used
func (o *Options) UseLocalDynamicPaths() bool {
	return o.LcBeforeKustomizeBuildPath != "" && o.LcAfterKustomizeBuildPath != "" && o.KustomizeBuildValues != ""
}

// InitializePathBuilder creates PathBuilder(s) from the new flags
func (o *Options) InitializePathBuilder() error {
	// Local mode with separate before/after paths
	if o.UseLocalDynamicPaths() {
		beforePb, err := pathbuilder.NewPathBuilder(o.LcBeforeKustomizeBuildPath, o.KustomizeBuildValues)
		if err != nil {
			return err
		}
		if err := beforePb.Validate(); err != nil {
			return err
		}
		o.BeforePathBuilder = beforePb

		afterPb, err := pathbuilder.NewPathBuilder(o.LcAfterKustomizeBuildPath, o.KustomizeBuildValues)
		if err != nil {
			return err
		}
		if err := afterPb.Validate(); err != nil {
			return err
		}
		o.AfterPathBuilder = afterPb
		return nil
	}

	// Shared path (GitHub mode or local with same structure)
	if o.UseDynamicPaths() {
		pb, err := pathbuilder.NewPathBuilder(o.KustomizeBuildPath, o.KustomizeBuildValues)
		if err != nil {
			return err
		}
		if err := pb.Validate(); err != nil {
			return err
		}
		o.PathBuilder = pb
	}
	return nil
}
