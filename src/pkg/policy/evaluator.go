package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/models"
	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry = log.New().WithFields(log.Fields{
	"package": "policy",
})

const (
	COMPLIANCE_CONFIG_FILENAME = "compliance-config.yaml"
)

// // PolicyEvaluator defines the interface for policy evaluation operations
// type PolicyEvaluator interface {
// 	// LoadAndValidate loads and validates the compliance configuration
// 	LoadAndValidate(configPath, policiesPath string) (*models.ComplianceConfig, error)
// 	// Evaluate evaluates all policies against the manifest
// 	Evaluate(ctx context.Context, manifest []byte, cfg *models.ComplianceConfig, policiesPath string) (*models.EvaluationResult, error)
// 	// CheckOverrides checks for policy override comments in PR comments
// 	CheckOverrides(comments []*models.Comment, cfg *models.ComplianceConfig) map[string]bool
// 	// Enforce determines if the evaluation result should block the PR
// 	Enforce(result *models.EvaluationResult, overrides map[string]bool) *models.EnforcementResult
// 	// ApplyOverrides applies policy overrides to the evaluation result
// 	ApplyOverrides(result *models.EvaluationResult, overrides map[string]bool)
// }

type PolicyEvaluatorInterface interface {
	LoadAndValidate(policiesPath string) (*models.ComplianceConfig, error)
	GeneratePolicyEvalResultForManifests(
		ctx context.Context,
		envManifests map[string][]byte,
		ghComments []string,
	) (*models.PolicyEvaluation, error)
}

const (
	POLICY_LEVEL_RECOMMEND     = "RECOMMEND"
	POLICY_LEVEL_WARNING       = "WARNING"
	POLICY_LEVEL_BLOCK         = "BLOCK"
	POLICY_LEVEL_OVERRIDE      = "OVERRIDE"
	POLICY_LEVEL_NOT_IN_EFFECT = "NOT_IN_EFFECT"
	POLICY_LEVEL_UNKNOWN       = ""
)

type EvaluatorData struct {
	models.ComplianceConfig

	// map policy id to full path to policy file
	fullPathToPolicy    map[string]string
	evalFailMsgOfPolicy map[string][]string

	// enforcements levels of policies Ids
	overrideCmdToPolicyId map[string]string
}

type PolicyEvaluator struct {
	policiesPath string
	data         EvaluatorData
}

func NewPolicyEvaluator(policiesPath string) *PolicyEvaluator {
	return &PolicyEvaluator{
		policiesPath: policiesPath,
		data: EvaluatorData{
			fullPathToPolicy:      make(map[string]string),
			evalFailMsgOfPolicy:   make(map[string][]string),
			overrideCmdToPolicyId: make(map[string]string),
		},
	}
}

// LoadAndValidate loads and validates the compliance configuration
func (e *PolicyEvaluator) LoadAndValidate() error {
	logger.Info("LoadAndValidate: starting...")

	// Load configuration
	logger.Info("LoadAndValidate: loading compliance configuration...")
	if err := e.loadComplianceConfig(); err != nil {
		return err
	}

	// Validate configuration structure
	logger.Info("LoadAndValidate: validating compliance configuration...")
	if err := e.validateComplianceConfig(); err != nil {
		return err
	}

	// Validate policy files exist and check for tests
	logger.Info("LoadAndValidate: validating policy files...")
	for id, policy := range e.data.ComplianceConfig.Policies {
		policyPath := filepath.Join(e.policiesPath, policy.FilePath)
		if _, err := os.Stat(policyPath); os.IsNotExist(err) {
			return fmt.Errorf("policy %s: file not found: %s", id, policyPath)
		}

		// Check for test file (support both .rego and .opa extensions)
		var testPath string
		if strings.HasSuffix(policyPath, ".rego") {
			testPath = strings.TrimSuffix(policyPath, ".rego") + "_test.rego"
		} else {
			return fmt.Errorf("policy %s: unsupported file extension (must be .rego)", id)
		}

		if _, err := os.Stat(testPath); os.IsNotExist(err) {
			return fmt.Errorf("each policy must have testpolicy %s: test file not found: %s", id, testPath)
		}

		// Set full path to policy file
		e.data.fullPathToPolicy[id] = policyPath

		// check override cmd
		if policy.Enforcement.Override.Comment == "" {
			continue
		}
		if _, ok := e.data.overrideCmdToPolicyId[policy.Enforcement.Override.Comment]; ok {
			return fmt.Errorf("policy %s: use another command, this override command already exists: %s", id, policy.Enforcement.Override.Comment)
		}
		e.data.overrideCmdToPolicyId[policy.Enforcement.Override.Comment] = id
	}

	logger.Infof("LoadAndValidate: done, loaded %d policies.", len(e.data.ComplianceConfig.Policies))
	return nil
}

// LoadComplianceConfig loads the compliance configuration from a YAML file
func (e *PolicyEvaluator) loadComplianceConfig() error {
	configPath := filepath.Join(e.policiesPath, COMPLIANCE_CONFIG_FILENAME)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read compliance config: %w", err)
	}

	if err := yaml.Unmarshal(data, &e.data.ComplianceConfig); err != nil {
		return fmt.Errorf("failed to parse compliance config: %w", err)
	}
	return nil
}

// ValidateComplianceConfig validates the common fields
func (e *PolicyEvaluator) validateComplianceConfig() error {
	if len(e.data.ComplianceConfig.Policies) == 0 {
		return fmt.Errorf("no policies defined in compliance config")
	}

	for id, policy := range e.data.ComplianceConfig.Policies {
		if policy.Name == "" {
			return fmt.Errorf("policy %s: name is required", id)
		}
		if policy.Type == "" {
			return fmt.Errorf("policy %s: type is required", id)
		}
		if policy.Type != "opa" {
			return fmt.Errorf("policy %s: unsupported type %s (only 'opa' is supported)", id, policy.Type)
		}
		if policy.FilePath == "" {
			return fmt.Errorf("policy %s: filePath is required", id)
		}

		// Validate enforcement dates are in order if set
		if policy.Enforcement.InEffectAfter != nil && policy.Enforcement.IsWarningAfter != nil {
			if policy.Enforcement.IsWarningAfter.Before(*policy.Enforcement.InEffectAfter) {
				return fmt.Errorf("policy %s: isWarningAfter cannot be before inEffectAfter", id)
			}
		}
		if policy.Enforcement.IsWarningAfter != nil && policy.Enforcement.IsBlockingAfter != nil {
			if policy.Enforcement.IsBlockingAfter.Before(*policy.Enforcement.IsWarningAfter) {
				return fmt.Errorf("policy %s: isBlockingAfter cannot be before isWarningAfter", id)
			}
		}

		// override comment not too long
		if policy.Enforcement.Override.Comment != "" && len(policy.Enforcement.Override.Comment) > 255 {
			return fmt.Errorf("policy %s: override comment is too long (max 255 characters)", id)
		}
	}

	return nil
}

func (e *PolicyEvaluator) GeneratePolicyEvalResultForManifests(
	ctx context.Context,
	build models.BuildManifestResult,
	ghComments []string,
) (
	*models.PolicyEvaluation,
	error,
) {
	logger.Info("GeneratePolicyEvalResultForManifests: starting...")

	envToPolicyIdToResult := make(map[string]map[string]models.PolicyResult)
	envManifests := build.EnvManifestBuild

	// 1. Evaluate policies for each environment and store results (can goroutine)
	complianceCfg := e.data.ComplianceConfig
	for env, manifest := range envManifests {
		// Skip policy evaluation if environment was skipped during build
		if manifest.Skipped {
			logger.WithField("env", env).WithField("reason", manifest.SkipReason).Info("Skipping policy evaluation for environment (overlay not found)")
			// Create empty results for skipped environments
			policyIdToResult := make(map[string]models.PolicyResult)
			for policyId := range complianceCfg.Policies {
				policy := complianceCfg.Policies[policyId]
				policyIdToResult[policyId] = models.PolicyResult{
					PolicyId:         policyId,
					PolicyName:       policy.Name,
					ExternalLink:     policy.ExternalLink,
					OverrideCommand:  policy.Enforcement.Override.Comment,
					IsPassing:        true, // Mark as passing since there's nothing to evaluate
					EvaluationStatus: "pass",
					FailMessages:     []string{},
				}
			}
			envToPolicyIdToResult[env] = policyIdToResult
			continue
		}

		logger.WithField("env", env).Info("Evaluating policies for environment")
		policyIdToResult := make(map[string]models.PolicyResult)

		evalResults, err := e.Evaluate(ctx, manifest.AfterManifest)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate policy for environment %s: %w", env, err)
		}

		for policyId, evalResult := range evalResults {
			logger.WithField("policyId", policyId).WithField("status", evalResult.Status).Debug("Evaluated policy")
			policy := complianceCfg.Policies[policyId]

			polResult := models.PolicyResult{
				PolicyId:         policyId,
				PolicyName:       policy.Name,
				ExternalLink:     policy.ExternalLink,
				OverrideCommand:  policy.Enforcement.Override.Comment,
				IsPassing:        len(evalResult.FailMessages) == 0,
				FailMessages:     evalResult.FailMessages,
				EvaluationStatus: evalResult.Status,
				ConfTestStdout:   evalResult.Stdout,
				ConfTestStderr:   evalResult.Stderr,
			}

			switch evalResult.Status {
			case "pass":
				polResult.IsPassing = true
				polResult.FailMessages = []string{}
			case "fail":
				polResult.IsPassing = false
				polResult.FailMessages = evalResult.FailMessages
			case "error":
				polResult.IsPassing = false
				polResult.ErrorMessage = evalResult.ErrorMessage
				polResult.FailMessages = []string{}
			}

			policyIdToResult[policyId] = polResult
		}

		envToPolicyIdToResult[env] = policyIdToResult
	}

	// 2. Get EnforcementLevel (can goroutine)
	policyIdToEnforcementLevel, err := e.DetermineEnforcementLevel(ghComments)
	if err != nil {
		return nil, fmt.Errorf("failed to determine enforcement level: %w", err)
	}

	// 3. Crafting PolicyEvaluation
	results := models.PolicyEvaluation{
		EnvironmentSummary: make(map[string]models.EnvironmentSummaryEnv),
		PolicyMatrix:       make(map[string]models.PolicyMatrix),
	}
	for env := range envManifests {
		logger.WithField("env", env).Info("Crafting policy evaluation for environment")

		totalCnt, failedCnt, erroredCnt, omittedCnt, successCnt := 0, 0, 0, 0, 0
		blockingSuccessCnt, warningSuccessCnt, recommendSuccessCnt, overriddenSuccessCnt, notInEffectSuccessCnt := 0, 0, 0, 0, 0
		blockingFailedCnt, warningFailedCnt, recommendFailedCnt, overriddenFailedCnt, notInEffectFailedCnt := 0, 0, 0, 0, 0
		blockingErroredCnt, warningErroredCnt, recommendErroredCnt, overriddenErroredCnt, notInEffectErroredCnt := 0, 0, 0, 0, 0

		blockingPolicies := []models.PolicyResult{}
		warningPolicies := []models.PolicyResult{}
		recommendPolicies := []models.PolicyResult{}
		overriddenPolicies := []models.PolicyResult{}
		notInEffectPolicies := []models.PolicyResult{}
		for policyId, result := range envToPolicyIdToResult[env] {
			totalCnt++

			// Count based on evaluation status
			switch result.EvaluationStatus {
			case "pass":
				successCnt++
			case "fail":
				failedCnt++
			case "error":
				erroredCnt++
			}

			enforcementLevel := policyIdToEnforcementLevel[policyId]
			switch enforcementLevel {
			case POLICY_LEVEL_BLOCK:
				blockingPolicies = append(blockingPolicies, result)
				switch result.EvaluationStatus {
				case "pass":
					blockingSuccessCnt++
				case "fail":
					blockingFailedCnt++
				case "error":
					blockingErroredCnt++
				}
			case POLICY_LEVEL_WARNING:
				warningPolicies = append(warningPolicies, result)
				switch result.EvaluationStatus {
				case "pass":
					warningSuccessCnt++
				case "fail":
					warningFailedCnt++
				case "error":
					warningErroredCnt++
				}
			case POLICY_LEVEL_RECOMMEND:
				recommendPolicies = append(recommendPolicies, result)
				switch result.EvaluationStatus {
				case "pass":
					recommendSuccessCnt++
				case "fail":
					recommendFailedCnt++
				case "error":
					recommendErroredCnt++
				}
			case POLICY_LEVEL_OVERRIDE:
				overriddenPolicies = append(overriddenPolicies, result)
				omittedCnt++
				switch result.EvaluationStatus {
				case "pass":
					overriddenSuccessCnt++
				case "fail":
					overriddenFailedCnt++
				case "error":
					overriddenErroredCnt++
				}
			case POLICY_LEVEL_NOT_IN_EFFECT:
				notInEffectPolicies = append(notInEffectPolicies, result)
				omittedCnt++
				switch result.EvaluationStatus {
				case "pass":
					notInEffectSuccessCnt++
				case "fail":
					notInEffectFailedCnt++
				case "error":
					notInEffectErroredCnt++
				}
			case POLICY_LEVEL_UNKNOWN:
				logger.Warnf("policy %s: unknown enforcement level: %s", policyId, enforcementLevel)
			}
		}
		results.PolicyMatrix[env] = models.PolicyMatrix{
			BlockingPolicies:    blockingPolicies,
			WarningPolicies:     warningPolicies,
			RecommendPolicies:   recommendPolicies,
			OverriddenPolicies:  overriddenPolicies,
			NotInEffectPolicies: notInEffectPolicies,
		}

		results.EnvironmentSummary[env] = models.EnvironmentSummaryEnv{
			PassingStatus: models.EnforcementPassingStatus{
				PassBlockingCheck:  blockingFailedCnt == 0 && blockingErroredCnt == 0,
				PassWarningCheck:   warningFailedCnt == 0 && warningErroredCnt == 0,
				PassRecommendCheck: recommendFailedCnt == 0 && recommendErroredCnt == 0,
			},
			PolicyCounts: models.PolicyCounts{
				TotalCount:          totalCnt,
				TotalSuccess:        successCnt,
				TotalFailed:         failedCnt,
				TotalErrored:        erroredCnt,
				TotalOmitted:        omittedCnt,
				TotalOmittedFailed:  overriddenFailedCnt + notInEffectFailedCnt,
				TotalOmittedSuccess: overriddenSuccessCnt + notInEffectSuccessCnt,

				BlockingSuccessCount:    blockingSuccessCnt,
				BlockingFailedCount:     blockingFailedCnt,
				BlockingErroredCount:    blockingErroredCnt,
				WarningSuccessCount:     warningSuccessCnt,
				WarningFailedCount:      warningFailedCnt,
				WarningErroredCount:     warningErroredCnt,
				RecommendSuccessCount:   recommendSuccessCnt,
				RecommendFailedCount:    recommendFailedCnt,
				RecommendErroredCount:   recommendErroredCnt,
				OverriddenSuccessCount:  overriddenSuccessCnt,
				OverriddenFailedCount:   overriddenFailedCnt,
				OverriddenErroredCount:  overriddenErroredCnt,
				NotInEffectSuccessCount: notInEffectSuccessCnt,
				NotInEffectFailedCount:  notInEffectFailedCnt,
				NotInEffectErroredCount: notInEffectErroredCnt,
			},
		}
	}

	return &results, nil
}

// Evaluate evaluates all policies against the manifest using conftest and store the evaluation results in the EvaluatorData
// returns: policyId -> models.PolicyEvalResult
func (e *PolicyEvaluator) Evaluate(
	ctx context.Context,
	manifest []byte,
) (map[string]models.PolicyEvalResult, error) {
	logger.Info("Evaluate: starting...")
	results := make(map[string]models.PolicyEvalResult)

	// Write manifest to temporary file for conftest
	tmpFile, err := os.CreateTemp("", "manifest-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if err := tmpFile.Close(); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close temp file: %v\n", err)
		}
		if err := os.Remove(tmpFile.Name()); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to remove temp file %s: %v\n", tmpFile.Name(), err)
		}
	}()

	if _, err := tmpFile.Write(manifest); err != nil {
		return nil, fmt.Errorf("failed to write manifest to temp file: %w", err)
	}

	// Evaluate each policy using conftest
	for id := range e.data.ComplianceConfig.Policies {
		evalResult := e.evaluatePolicyWithConftest(
			ctx, id, e.data.fullPathToPolicy[id], tmpFile.Name(),
		)
		results[id] = evalResult
	}

	return results, nil
}

// evaluatePolicyWithConftest evaluates a single policy using conftest
// returns: models.PolicyEvalResult
func (e *PolicyEvaluator) evaluatePolicyWithConftest(
	ctx context.Context,
	id string,
	singlePolicyPath string, manifestPath string,
) models.PolicyEvalResult {
	logger.Infof("evaluating policy %s", id)

	cmd := exec.CommandContext(ctx,
		"conftest", "test", "--all-namespaces", "--combine",
		"--policy", singlePolicyPath,
		manifestPath,
		"-o", "json",
	)

	// Separate stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command and capture exit code
	cmdErr := cmd.Run()
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	logger.Debugf("conftest stdout: %s", stdoutStr)
	if stderrStr != "" {
		logger.Debugf("conftest stderr: %s", stderrStr)
	}

	// Check if conftest binary execution failed (not just policy failures)
	// Note: conftest exits with code 1 when policies fail, but still outputs valid JSON
	// It exits with other codes when there's an actual error (e.g., invalid policy syntax)
	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok && exitErr.ExitCode() != 1 {
			// Non-1 exit code indicates actual error (not policy failure)
			errMsg := fmt.Sprintf("conftest execution error (exit code %d): %v", exitErr.ExitCode(), cmdErr)
			logger.Errorf("Policy %s: %s", id, errMsg)
			logger.Errorf("Policy %s stdout: %s", id, stdoutStr)
			logger.Errorf("Policy %s stderr: %s", id, stderrStr)
			return models.PolicyEvalResult{
				Status:       "error",
				ErrorMessage: errMsg,
				Stdout:       stdoutStr,
				Stderr:       stderrStr,
			}
		}
		// Exit code 1 is normal for policy failures, continue to parse JSON
	}

	// Sample conftest output
	// 	[
	//   {
	//     "filename": "Combined",
	//     "namespace": "main",
	//     "successes": 2,
	//     "failures": [
	//       {
	//         "msg": "Deployment 'prod-my-app' must have at least 2 replicas for high availability, found: 1",
	//         "metadata": {
	//           "query": "data.main.deny"
	//         }
	//       }
	//     ]
	//   }
	// ]
	outputJson := []struct {
		Filename  string `json:"filename"`
		Namespace string `json:"namespace"`
		Successes int    `json:"successes"`
		Failures  []struct {
			Msg      string `json:"msg"`
			Metadata struct {
				Query string `json:"query"`
			}
		}
	}{}

	if err := json.Unmarshal([]byte(stdoutStr), &outputJson); err != nil {
		// JSON parsing failed - this is an error condition
		errMsg := fmt.Sprintf("failed to parse conftest JSON output: %v", err)
		logger.Errorf("Policy %s: %s", id, errMsg)
		logger.Errorf("Policy %s stdout: %s", id, stdoutStr)
		logger.Errorf("Policy %s stderr: %s", id, stderrStr)
		return models.PolicyEvalResult{
			Status:       "error",
			ErrorMessage: errMsg,
			Stdout:       stdoutStr,
			Stderr:       stderrStr,
		}
	}

	if len(outputJson) == 0 {
		errMsg := "no results found in conftest output"
		logger.Errorf("Policy %s: %s", id, errMsg)
		logger.Errorf("Policy %s stdout: %s", id, stdoutStr)
		logger.Errorf("Policy %s stderr: %s", id, stderrStr)
		return models.PolicyEvalResult{
			Status:       "error",
			ErrorMessage: errMsg,
			Stdout:       stdoutStr,
			Stderr:       stderrStr,
		}
	}

	// Success case: [
	// 	 {
	// 			"filename": "Combined",
	// 			"namespace": "main",
	// 			"successes": 3
	//	 }
	// ]
	if len(outputJson[0].Failures) == 0 {
		return models.PolicyEvalResult{
			Status: "pass",
			Stdout: stdoutStr,
			Stderr: stderrStr,
		}
	}

	failureMsgs := []string{}
	for _, failure := range outputJson[0].Failures {
		failureMsgs = append(failureMsgs, failure.Msg)
	}
	return models.PolicyEvalResult{
		Status:       "fail",
		FailMessages: failureMsgs,
		Stdout:       stdoutStr,
		Stderr:       stderrStr,
	}
}

// DetermineEnforcementLevel determines the current enforcement level based on time and overrides
// Set the results to internal struct data
func (e *PolicyEvaluator) DetermineEnforcementLevel(
	comments []string,
) (map[string]string, error) {
	results := make(map[string]string)
	now := time.Now()

	for _, comment := range comments {
		if _, ok := e.data.overrideCmdToPolicyId[comment]; ok {
			results[e.data.overrideCmdToPolicyId[comment]] = POLICY_LEVEL_OVERRIDE
		}
	}

	for policyId, policy := range e.data.ComplianceConfig.Policies {
		if _, ok := results[policyId]; ok {
			continue // already set during OVERRIDE checks
		}

		enforcementLevel := POLICY_LEVEL_UNKNOWN
		enforcement := policy.Enforcement

		if enforcement.InEffectAfter != nil && now.Before(*enforcement.InEffectAfter) {
			enforcementLevel = POLICY_LEVEL_NOT_IN_EFFECT
		}
		if enforcement.InEffectAfter != nil && !now.Before(*enforcement.InEffectAfter) {
			enforcementLevel = POLICY_LEVEL_RECOMMEND
		}
		if enforcement.IsWarningAfter != nil && !now.Before(*enforcement.IsWarningAfter) {
			enforcementLevel = POLICY_LEVEL_WARNING
		}
		if enforcement.IsBlockingAfter != nil && !now.Before(*enforcement.IsBlockingAfter) {
			enforcementLevel = POLICY_LEVEL_BLOCK
		}

		results[policyId] = enforcementLevel
	}

	return results, nil
}
