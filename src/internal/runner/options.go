package runner

type Options struct {
	// Run mode
	RunMode string // "github" or "local"
	Debug   bool   // Debug mode

	// Common options
	Service                       string
	Environments                  []string // Support multiple environments
	PoliciesPath                  string
	TemplatesPath                 string
	OutputDir                     string
	EnableExportReport            bool
	EnableExportPerformanceReport bool
	FailOnOverlayNotFound         bool // Fail if overlay doesn't exist (default: false, skip gracefully)

	// GitHub mode options
	GhRepo        string
	GhPrNumber    int
	ManifestsPath string // Path to services directory (default: ./services)

	// Local mode options
	LcBeforeManifestsPath string
	LcAfterManifestsPath  string
}
