package models

type BuildManifestResult struct {
	// EnvManifestBuild maps overlay keys to build results
	// For legacy mode: key = environment name (e.g., "stg", "prod")
	// For dynamic mode: key = overlay key (e.g., "alpha/stg", "my-app/alpha/stg")
	EnvManifestBuild map[string]BuildEnvManifestResult

	// OverlayKeys preserves the ordered list of overlay keys
	// This maintains the order specified in --kustomize-build-values or --environments
	OverlayKeys []string
}

type BuildEnvManifestResult struct {
	// OverlayKey is the unique identifier for this build
	// For legacy mode: same as Environment
	// For dynamic mode: combined variable values (e.g., "alpha/stg")
	OverlayKey string

	// Environment is kept for backward compatibility (same as OverlayKey)
	Environment string

	// FullBuildPath is the actual path used for kustomize build (only for dynamic mode)
	FullBuildPath string

	BeforeManifest []byte
	AfterManifest  []byte
	Skipped        bool   // true if overlay doesn't exist and was skipped
	SkipReason     string // reason for skipping (e.g., "overlay not found")
}

type PolicyEvaluateResult struct {
	EnvPolicyEvaluate map[string]PolicyEnvEvaluateResult
}

type PolicyEnvEvaluateResult struct {
	// OverlayKey is the unique identifier (same as key in the map)
	OverlayKey string

	// Environment is kept for backward compatibility
	Environment string

	// if key exists, value is not empty => failed
	// if key exists, value empty => passed
	PolicyIdToEvalFailMsgs map[string][]string
}
