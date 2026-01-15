package models

import "time"

// ComplianceConfig represents the complete compliance configuration
// - Policies: id -> PolicyConfig
// - PolicyIDs: ordered list of policy IDs (preserves YAML order)
type ComplianceConfig struct {
	Policies  map[string]PolicyConfig `yaml:"policies"`
	PolicyIDs []string                `yaml:"-"` // Not in YAML, populated during load
}

// PolicyConfig represents a single policy configuration
type PolicyConfig struct {
	Name         string            `yaml:"name"`
	Description  string            `yaml:"description"`
	Type         string            `yaml:"type"` // "opa" only for now
	FilePath     string            `yaml:"filePath"`
	ExternalLink string            `yaml:"externalLink,omitempty"` // Optional link to policy documentation
	Enforcement  EnforcementConfig `yaml:"enforcement"`
}

// EnforcementConfig defines when and how a policy should be enforced
type EnforcementConfig struct {
	InEffectAfter   *time.Time     `yaml:"inEffectAfter,omitempty"`
	IsWarningAfter  *time.Time     `yaml:"isWarningAfter,omitempty"`
	IsBlockingAfter *time.Time     `yaml:"isBlockingAfter,omitempty"`
	Override        OverrideConfig `yaml:"override"`
}

// OverrideConfig defines how a policy can be overridden
type OverrideConfig struct {
	Comment string `yaml:"comment"` // e.g., "/sp-override-ha"
}
