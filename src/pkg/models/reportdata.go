package models

import "time"

// ReportData represents the complete report data structure
type ReportData struct {
	Service      string    `json:"service"`
	Timestamp    time.Time `json:"timestamp"`
	BaseCommit   string    `json:"baseCommit"`
	HeadCommit   string    `json:"headCommit"`
	Environments []string  `json:"environments"`

	// Manifest changes per environment
	ManifestChanges map[string]EnvironmentDiff `json:"manifestChanges"`

	// Policy evaluation results
	PolicyEvaluation PolicyEvaluation `json:"policyEvaluation"`
}

// EnvironmentDiff represents diff data for a single environment
type EnvironmentDiff struct {
	LineCount        int `json:"lineCount"`
	AddedLineCount   int `json:"addedLineCount"`
	DeletedLineCount int `json:"deletedLineCount"`

	ContentGHFilePath *string `json:"contentGHFilePath"` // file path in the runner's output directory if the diff is too long
	ContentType       string  `json:"contentType"`       // "text" or "ext_ghartifact"
	Content           string  `json:"content"`           // diff text OR artifact URL
}

// PolicyEvaluationSummary represents the overall policy evaluation results
type PolicyEvaluation struct {
	// Summary table: Environment -> Success/Failed/Errored counts
	EnvironmentSummary map[string]EnvironmentSummaryEnv `json:"environmentSummary"`

	// Detailed policy matrix
	PolicyMatrix map[string]PolicyMatrix `json:"policyMatrix"`
}

type EnvironmentSummaryEnv struct {
	PassingStatus EnforcementPassingStatus `json:"passingStatus"`
	PolicyCounts  PolicyCounts             `json:"policyCounts"`
}

type EnforcementPassingStatus struct {
	PassBlockingCheck  bool `json:"passBlockingCheck"`
	PassWarningCheck   bool `json:"passWarningCheck"`
	PassRecommendCheck bool `json:"passRecommendCheck"`
}

// PolicyCounts represents the count of policies by status for an environment
type PolicyCounts struct {
	TotalCount          int `json:"totalCount"`
	TotalSuccess        int `json:"totalSuccess"`        // total number of policies that passed
	TotalFailed         int `json:"totalFailed"`         // total number of policies of level RECOMMEND, WARNING, BLOCKING that failed
	TotalErrored        int `json:"totalErrored"`        // total number of policies that had evaluation errors
	TotalOmitted        int `json:"totalOmitted"`        // total number of policies of level OVERRIDE, NOT_IN_EFFECT that either failed or passed
	TotalOmittedFailed  int `json:"totalOmittedFailed"`  // total number of policies of level OVERRIDE, NOT_IN_EFFECT that failed
	TotalOmittedSuccess int `json:"totalOmittedSuccess"` // total number of policies of level OVERRIDE, NOT_IN_EFFECT that passed

	BlockingSuccessCount    int `json:"blockingSuccessCount"`
	BlockingFailedCount     int `json:"blockingFailedCount"`
	BlockingErroredCount    int `json:"blockingErroredCount"`
	WarningSuccessCount     int `json:"warningSuccessCount"`
	WarningFailedCount      int `json:"warningFailedCount"`
	WarningErroredCount     int `json:"warningErroredCount"`
	RecommendSuccessCount   int `json:"recommendSuccessCount"`
	RecommendFailedCount    int `json:"recommendFailedCount"`
	RecommendErroredCount   int `json:"recommendErroredCount"`
	OverriddenSuccessCount  int `json:"overriddenSuccessCount"`
	OverriddenFailedCount   int `json:"overriddenFailedCount"`
	OverriddenErroredCount  int `json:"overriddenErroredCount"`
	NotInEffectSuccessCount int `json:"notInEffectSuccessCount"`
	NotInEffectFailedCount  int `json:"notInEffectFailedCount"`
	NotInEffectErroredCount int `json:"notInEffectErroredCount"`
}

// PolicyMatrix represents the detailed policy evaluation matrix
type PolicyMatrix struct {
	// Policies grouped by enforcement level
	BlockingPolicies    []PolicyResult `json:"blockingPolicies"`
	WarningPolicies     []PolicyResult `json:"warningPolicies"`
	RecommendPolicies   []PolicyResult `json:"recommendPolicies"`
	OverriddenPolicies  []PolicyResult `json:"overriddenPolicies"`
	NotInEffectPolicies []PolicyResult `json:"notInEffectPolicies"`
}

// PolicyResult represents the result of a single policy evaluation
type PolicyResult struct {
	PolicyId          string   `json:"policyId"`
	PolicyName        string   `json:"policyName"`
	ExternalLink      string   `json:"externalLink,omitempty"` // Optional link to policy documentation
	IsPassing         bool     `json:"isPassing"`              // true if policy passed, false if failed or errored
	EvaluationStatus  string   `json:"evaluationStatus"`       // "pass", "fail", or "error"
	FailMessages      []string `json:"failMessages"`           // Policy violation messages (for "fail" status)
	ErrorMessage      string   `json:"errorMessage,omitempty"` // Error details (for "error" status)
	ConfTestStdout    string   `json:"-"`                      // Not exported to JSON, for internal debugging
	ConfTestStderr    string   `json:"-"`                      // Not exported to JSON, for internal debugging
}

// ReportTemplateData represents the data structure for template rendering
type ReportTemplateData struct {
	ReportData
	RenderedMarkdown string `json:"renderedMarkdown"`
}
