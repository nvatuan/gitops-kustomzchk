package models

type BuildManifestResult struct {
	EnvManifestBuild map[string]BuildEnvManifestResult
}

type BuildEnvManifestResult struct {
	Environment    string
	BeforeManifest []byte
	AfterManifest  []byte
	Skipped        bool   // true if overlay doesn't exist and was skipped
	SkipReason     string // reason for skipping (e.g., "overlay not found")
}

type PolicyEvaluateResult struct {
	EnvPolicyEvaluate map[string]PolicyEnvEvaluateResult
}

type PolicyEnvEvaluateResult struct {
	Environment string

	// Maps policy ID to evaluation result
	PolicyIdToEvalResult map[string]PolicyEvalResult
}

// PolicyEvalResult represents the result of evaluating a single policy with conftest
type PolicyEvalResult struct {
	Status       string   // "pass", "fail", or "error"
	FailMessages []string // Policy violation messages (for "fail" status)
	ErrorMessage string   // Error details (for "error" status)
	Stdout       string   // conftest stdout (for debugging)
	Stderr       string   // conftest stderr (for debugging)
}
