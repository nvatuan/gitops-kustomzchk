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

	// if key exists, value is not empty => failed
	// if key exists, value empty => passed
	PolicyIdToEvalFailMsgs map[string][]string
}
