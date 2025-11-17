package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/diff"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/kustomize"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/models"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/policy"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/template"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/trace"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry = log.New().WithFields(log.Fields{
	"package": "runner",
})

type RunnerBase struct {
	Context context.Context
	Options *Options

	RunMode string

	Builder   *kustomize.Builder
	Differ    *diff.Differ
	Evaluator *policy.PolicyEvaluator
	Renderer  *template.Renderer

	Instance RunnerInterface
}

// make RunnerLocal implement RunnerInterface
var _ RunnerInterface = (*RunnerBase)(nil)

func NewRunnerBase(
	ctx context.Context,
	options *Options,
	builder *kustomize.Builder,
	differ *diff.Differ,
	evaluator *policy.PolicyEvaluator,
	renderer *template.Renderer,
) (*RunnerBase, error) {
	runner := &RunnerBase{
		Context:   ctx,
		Options:   options,
		RunMode:   options.RunMode,
		Builder:   builder,
		Differ:    differ,
		Evaluator: evaluator,
		Renderer:  renderer,
	}
	return runner, nil
}

func (r *RunnerBase) Initialize() error {
	logger.Info("Initializing runner: starting...")

	// if any is nil, return error
	if r.Builder == nil || r.Differ == nil || r.Evaluator == nil || r.Renderer == nil {
		return fmt.Errorf("builder, differ, evaluator, reporter, and renderer are required")
	}

	logger.Info("Initalize runner: Evaluator: Loading and validating policy configuration")
	// load and validate policy configuration
	err := r.Evaluator.LoadAndValidate()
	if err != nil {
		return fmt.Errorf("failed to load policy config: %w", err)
	}

	logger.Info("Initalize runner: done.")
	return nil
}

func (r *RunnerBase) BuildManifests(beforePath, afterPath string) (*models.BuildManifestResult, error) {
	ctx, span := trace.StartSpan(r.Context, "BuildManifests")
	defer span.End()

	logger.Info("BuildManifests: starting...")

	results := make(map[string]models.BuildEnvManifestResult)
	envs := r.Options.Environments
	for _, env := range envs {
		envCtx, envSpan := trace.StartSpan(ctx, fmt.Sprintf("BuildManifests.%s", env))

		logger.WithField("env", env).WithField("beforePath", beforePath).Info("Building before manifest...")
		beforeManifest, err := r.Builder.Build(envCtx, beforePath, env)
		if err != nil {
			if errors.Is(err, kustomize.ErrOverlayNotFound) {
				// Overlay doesn't exist, mark as skipped
				logger.WithField("env", env).Warn("Environment overlay not found for before path, marking as skipped")
				results[env] = models.BuildEnvManifestResult{
					Environment: env,
					Skipped:     true,
					SkipReason:  "overlay not found in before path",
				}
				envSpan.End()
				continue
			}
			envSpan.End()
			return nil, err
		}

		logger.WithField("env", env).WithField("afterPath", afterPath).Info("Building after manifest...")
		afterManifest, err := r.Builder.Build(envCtx, afterPath, env)
		if err != nil {
			if errors.Is(err, kustomize.ErrOverlayNotFound) {
				// Overlay doesn't exist, mark as skipped
				logger.WithField("env", env).Warn("Environment overlay not found for after path, marking as skipped")
				results[env] = models.BuildEnvManifestResult{
					Environment: env,
					Skipped:     true,
					SkipReason:  "overlay not found in after path",
				}
				envSpan.End()
				continue
			}
			envSpan.End()
			return nil, err
		}
		results[env] = models.BuildEnvManifestResult{
			Environment:    env,
			BeforeManifest: beforeManifest,
			AfterManifest:  afterManifest,
			Skipped:        false,
		}
		logger.WithField("env", env).WithField("beforeManifest", string(beforeManifest)).Debug("Built Manifest")
		logger.WithField("env", env).WithField("afterManifest", string(afterManifest)).Debug("Built Manifest")

		envSpan.End()
	}

	logger.Info("BuildManifests: done.")
	return &models.BuildManifestResult{
		EnvManifestBuild: results,
	}, nil
}

func (r *RunnerBase) DiffManifests(result *models.BuildManifestResult) (map[string]models.EnvironmentDiff, error) {
	ctx, span := trace.StartSpan(r.Context, "DiffManifests")
	defer span.End()

	logger.Info("DiffManifests: starting...")

	results := make(map[string]models.EnvironmentDiff)

	for env, envResult := range result.EnvManifestBuild {
		_, envSpan := trace.StartSpan(ctx, fmt.Sprintf("DiffManifests.%s", env))

		// Skip diff if environment was skipped during build
		if envResult.Skipped {
			logger.WithField("env", env).WithField("reason", envResult.SkipReason).Info("Skipping diff for environment")
			results[env] = models.EnvironmentDiff{
				ContentType: models.DiffContentTypeText,
				Content:     fmt.Sprintf("Environment skipped: %s", envResult.SkipReason),
			}
			envSpan.End()
			continue
		}

		diffContent, err := r.Differ.Diff(envResult.BeforeManifest, envResult.AfterManifest)
		if err != nil {
			logger.WithField("env", envResult.Environment).WithField("error", err).Error("Failed to diff manifests")
			envSpan.End()
			return nil, err
		}
		logger.WithField("env", envResult.Environment).WithField("diffContent", diffContent).Debug("Diffed Manifest")

		addedLines, deletedLines, totalLines := diff.CalcLineChangesFromDiffContent(diffContent)
		results[env] = models.EnvironmentDiff{
			ContentType:      models.DiffContentTypeText,
			LineCount:        totalLines,
			AddedLineCount:   addedLines,
			DeletedLineCount: deletedLines,
			Content:          diffContent,
		}

		envSpan.End()
	}

	logger.Info("DiffManifests: done.")
	return results, nil
}

func (r *RunnerBase) EvaluatePolicies(mf *models.BuildManifestResult) (*models.PolicyEvaluateResult, error) {
	ctx, span := trace.StartSpan(r.Context, "EvaluatePolicies")
	defer span.End()
	logger.Info("EvaluatePolicies: starting...")

	results := models.PolicyEvaluateResult{
		EnvPolicyEvaluate: make(map[string]models.PolicyEnvEvaluateResult),
	}

	for _, envResult := range mf.EnvManifestBuild {
		_, envSpan := trace.StartSpan(ctx, fmt.Sprintf("EvaluatePolicies.%s", envResult.Environment))

		// Skip policy evaluation if environment was skipped during build
		if envResult.Skipped {
			logger.WithField("env", envResult.Environment).WithField("reason", envResult.SkipReason).Info("Skipping policy evaluation for environment")
			// Store empty result to indicate it was skipped
			results.EnvPolicyEvaluate[envResult.Environment] = models.PolicyEnvEvaluateResult{
				Environment:            envResult.Environment,
				PolicyIdToEvalFailMsgs: make(map[string][]string),
			}
			envSpan.End()
			continue
		}

		// only evaluate the after manifest
		envManifest := envResult.AfterManifest
		failMsgs, err := r.Evaluator.Evaluate(ctx, envManifest)
		if err != nil {
			envSpan.End()
			return nil, err
		}
		results.EnvPolicyEvaluate[envResult.Environment] = models.PolicyEnvEvaluateResult{
			Environment:            envResult.Environment,
			PolicyIdToEvalFailMsgs: failMsgs,
		}

		envSpan.End()
	}

	logger.Info("EvaluatePolicies: done.")
	return &results, nil
}

func (r *RunnerBase) Process() error {
	_, span := trace.StartSpan(r.Context, "Process")
	defer span.End()
	logger.Info("Process: starting...")

	beforePath := filepath.Join(r.Options.LcBeforeManifestsPath, r.Options.Service)
	afterPath := filepath.Join(r.Options.LcAfterManifestsPath, r.Options.Service)
	rs, err := r.BuildManifests(beforePath, afterPath)
	if err != nil {
		return err
	}
	logger.WithField("results", rs).Debug("Built Manifests")

	diffs, err := r.DiffManifests(rs)
	if err != nil {
		return err
	}
	logger.WithField("results", diffs).Debug("Diffed Manifests")

	policyEval, err := r.Evaluator.GeneratePolicyEvalResultForManifests(r.Context, *rs, []string{})
	if err != nil {
		return err
	}
	logger.WithField("results", policyEval).Debug("Evaluated Policies")

	reportData := models.ReportData{
		Service:          r.Options.Service,
		Timestamp:        time.Now(),
		BaseCommit:       "base",
		HeadCommit:       "head",
		Environments:     r.Options.Environments,
		ManifestChanges:  diffs,
		PolicyEvaluation: *policyEval,
	}

	if err := r.Output(&reportData); err != nil {
		return err
	}
	return nil
}

func (r *RunnerBase) Output(data *models.ReportData) error {
	_, span := trace.StartSpan(r.Context, "Output")
	defer span.End()

	logger.Info("Output: starting...")
	if err := r.outputReportJson(data); err != nil {
		return err
	}
	logger.Info("Output: done.")
	return nil
}

// Exporting report json file to output directory if enabled
func (r *RunnerBase) outputReportJson(data *models.ReportData) error {
	if !r.Options.EnableExportReport {
		logger.Info("OutputJson: option was disabled")
		return nil
	}
	logger.Info("OutputJson: starting...")

	if err := os.MkdirAll(r.Options.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	resultsJson, err := json.Marshal(data)
	if err != nil {
		return err
	}
	filePath := filepath.Join(r.Options.OutputDir, "report.json")
	if err := os.WriteFile(filePath, resultsJson, 0644); err != nil {
		logger.WithField("filePath", filePath).WithField("error", err).Error("Failed to write report data to file")
		return err
	}
	logger.WithField("filePath", filePath).Info("Written report data to file")
	return nil
}
