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
)

type RunnerLocal struct {
	RunnerBase
}

// make RunnerLocal implement RunnerInterface
var _ RunnerInterface = (*RunnerLocal)(nil)

func NewRunnerLocal(
	ctx context.Context,
	options *Options,
	builder *kustomize.Builder,
	differ *diff.Differ,
	evaluator *policy.PolicyEvaluator,
	renderer *template.Renderer,
) (*RunnerLocal, error) {
	baseRunner, err := NewRunnerBase(ctx, options, builder, differ, evaluator, renderer)
	if err != nil {
		return nil, err
	}
	runner := &RunnerLocal{
		RunnerBase: *baseRunner,
	}
	return runner, nil
}

func (r *RunnerLocal) Initialize() error {
	return r.RunnerBase.Initialize()
}

func (r *RunnerLocal) BuildManifests(beforePath, afterPath string) (*models.BuildManifestResult, error) {
	return r.RunnerBase.BuildManifests(beforePath, afterPath)
}

func (r *RunnerLocal) DiffManifests(result *models.BuildManifestResult) (map[string]models.EnvironmentDiff, error) {
	return r.RunnerBase.DiffManifests(result)
}

func (r *RunnerLocal) Process() error {
	ctx, span := trace.StartSpan(r.Context, "Process")
	defer span.End()

	logger.Info("Process: starting...")

	var rs *models.BuildManifestResult
	var err error

	if r.Options.UseLocalDynamicPaths() {
		// Local dynamic mode with separate before/after path templates
		rs, err = r.buildManifestsLocalDynamic(ctx)
	} else if r.Options.UseDynamicPaths() {
		// Shared dynamic mode: use the before/after paths directly as roots
		beforePath := r.Options.LcBeforeManifestsPath
		afterPath := r.Options.LcAfterManifestsPath
		rs, err = r.BuildManifests(beforePath, afterPath)
	} else {
		// Legacy mode: append service name to paths
		beforePath := filepath.Join(r.Options.LcBeforeManifestsPath, r.Options.Service)
		afterPath := filepath.Join(r.Options.LcAfterManifestsPath, r.Options.Service)
		rs, err = r.BuildManifests(beforePath, afterPath)
	}

	if err != nil {
		return err
	}
	logger.WithField("results", rs).Debug("Built Manifests")

	diffs, err := r.DiffManifests(rs)
	if err != nil {
		return err
	}
	logger.WithField("results", diffs).Debug("Diffed Manifests")

	_, evalSpan := trace.StartSpan(ctx, "EvaluatePolicies")
	policyEval, err := r.Evaluator.GeneratePolicyEvalResultForManifests(ctx, *rs, []string{})
	if err != nil {
		evalSpan.End()
		return err
	}
	evalSpan.End()
	logger.WithField("results", policyEval).Debug("Evaluated Policies")

	// Build report data
	reportData := r.buildReportData(rs, diffs, policyEval)

	if err := r.Output(&reportData); err != nil {
		return err
	}
	return nil
}

// buildManifestsLocalDynamic handles local mode with separate before/after path templates
func (r *RunnerLocal) buildManifestsLocalDynamic(ctx context.Context) (*models.BuildManifestResult, error) {
	_, span := trace.StartSpan(ctx, "BuildManifestsLocalDynamic")
	defer span.End()

	logger.Info("BuildManifestsLocalDynamic: starting...")

	// Generate paths from before path builder
	beforeCombos, err := r.Options.BeforePathBuilder.GenerateAllPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to generate before path combinations: %w", err)
	}

	// Generate paths from after path builder
	afterCombos, err := r.Options.AfterPathBuilder.GenerateAllPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to generate after path combinations: %w", err)
	}

	// Create a map of after paths by overlay key for quick lookup
	afterPathMap := make(map[string]string)
	for _, combo := range afterCombos {
		afterPathMap[combo.OverlayKey] = combo.Path
	}

	results := make(map[string]models.BuildEnvManifestResult)

	for _, beforeCombo := range beforeCombos {
		overlayKey := beforeCombo.OverlayKey
		comboCtx, comboSpan := trace.StartSpan(ctx, fmt.Sprintf("BuildManifests.%s", overlayKey))

		afterPath, exists := afterPathMap[overlayKey]
		if !exists {
			logger.WithField("overlayKey", overlayKey).Warn("No matching after path for overlay key, skipping")
			results[overlayKey] = models.BuildEnvManifestResult{
				OverlayKey:  overlayKey,
				Environment: overlayKey,
				Skipped:     true,
				SkipReason:  "no matching after path",
			}
			comboSpan.End()
			continue
		}

		logger.WithField("overlayKey", overlayKey).WithField("beforePath", beforeCombo.Path).Info("Building before manifest...")
		beforeManifest, err := r.Builder.BuildAtFullPath(comboCtx, beforeCombo.Path)
		if err != nil {
			if errors.Is(err, kustomize.ErrOverlayNotFound) {
				logger.WithField("overlayKey", overlayKey).Warn("Overlay not found for before path, marking as skipped")
				results[overlayKey] = models.BuildEnvManifestResult{
					OverlayKey:    overlayKey,
					Environment:   overlayKey,
					FullBuildPath: beforeCombo.Path,
					Skipped:       true,
					SkipReason:    "overlay not found in before path",
				}
				comboSpan.End()
				continue
			}
			comboSpan.End()
			return nil, err
		}

		logger.WithField("overlayKey", overlayKey).WithField("afterPath", afterPath).Info("Building after manifest...")
		afterManifest, err := r.Builder.BuildAtFullPath(comboCtx, afterPath)
		if err != nil {
			if errors.Is(err, kustomize.ErrOverlayNotFound) {
				logger.WithField("overlayKey", overlayKey).Warn("Overlay not found for after path, marking as skipped")
				results[overlayKey] = models.BuildEnvManifestResult{
					OverlayKey:    overlayKey,
					Environment:   overlayKey,
					FullBuildPath: afterPath,
					Skipped:       true,
					SkipReason:    "overlay not found in after path",
				}
				comboSpan.End()
				continue
			}
			comboSpan.End()
			return nil, err
		}

		results[overlayKey] = models.BuildEnvManifestResult{
			OverlayKey:     overlayKey,
			Environment:    overlayKey,
			FullBuildPath:  afterPath, // Store the after path
			BeforeManifest: beforeManifest,
			AfterManifest:  afterManifest,
			Skipped:        false,
		}
		logger.WithField("overlayKey", overlayKey).Debug("Built Manifest")

		comboSpan.End()
	}

	logger.Info("BuildManifestsLocalDynamic: done.")
	return &models.BuildManifestResult{
		EnvManifestBuild: results,
	}, nil
}

// buildReportData constructs the ReportData based on legacy or dynamic mode
func (r *RunnerLocal) buildReportData(
	rs *models.BuildManifestResult,
	diffs map[string]models.EnvironmentDiff,
	policyEval *models.PolicyEvaluation,
) models.ReportData {
	reportData := models.ReportData{
		Timestamp:        time.Now(),
		BaseCommit:       "base",
		HeadCommit:       "head",
		ManifestChanges:  diffs,
		PolicyEvaluation: *policyEval,
	}

	if r.Options.UseLocalDynamicPaths() {
		// Local dynamic mode with separate before/after paths
		reportData.KustomizeBuildValues = r.Options.KustomizeBuildValues

		// Extract overlay keys from results
		overlayKeys := make([]string, 0, len(rs.EnvManifestBuild))
		for key := range rs.EnvManifestBuild {
			overlayKeys = append(overlayKeys, key)
		}
		reportData.OverlayKeys = overlayKeys
		reportData.Environments = overlayKeys // For backward compat in templates
	} else if r.Options.UseDynamicPaths() {
		// Shared dynamic mode
		reportData.KustomizeBuildPath = r.Options.KustomizeBuildPath
		reportData.KustomizeBuildValues = r.Options.KustomizeBuildValues

		// Extract overlay keys from results
		overlayKeys := make([]string, 0, len(rs.EnvManifestBuild))
		for key := range rs.EnvManifestBuild {
			overlayKeys = append(overlayKeys, key)
		}
		reportData.OverlayKeys = overlayKeys
		reportData.Environments = overlayKeys // For backward compat in templates
	} else {
		// Legacy mode
		reportData.Service = r.Options.Service
		reportData.Environments = r.Options.Environments
		reportData.OverlayKeys = r.Options.Environments // For consistency
	}

	return reportData
}

func (r *RunnerLocal) Output(data *models.ReportData) error {
	_, span := trace.StartSpan(r.Context, "Output")
	defer span.End()

	logger.Info("Output: starting...")
	if err := r.outputReportJson(data); err != nil {
		return err
	}
	if err := r.outputReportMarkdown(data); err != nil {
		return err
	}
	logger.Info("Output: done.")
	return nil
}

// Exporting report json file to output directory if enabled
func (r *RunnerLocal) outputReportJson(data *models.ReportData) error {
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

// Exporting report markdown file to output directory
func (r *RunnerLocal) outputReportMarkdown(data *models.ReportData) error {
	logger.Info("OutputMarkdown: starting...")

	// Render the markdown using templates
	renderedMarkdown, err := r.Renderer.RenderWithTemplates(r.Options.TemplatesPath, data)
	if err != nil {
		logger.WithField("error", err).Error("Failed to render markdown template")
		return err
	}

	// Write the rendered markdown to file
	filePath := filepath.Join(r.Options.OutputDir, "report.md")
	if err := os.WriteFile(filePath, []byte(renderedMarkdown), 0644); err != nil {
		logger.WithField("filePath", filePath).WithField("error", err).Error("Failed to write markdown report to file")
		return err
	}

	logger.WithField("filePath", filePath).Info("Written markdown report to file")
	return nil
}
