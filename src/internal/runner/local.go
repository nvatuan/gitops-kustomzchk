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

	// Create a map of before paths by overlay key for quick lookup
	beforePathMap := make(map[string]string)
	for _, combo := range beforeCombos {
		beforePathMap[combo.OverlayKey] = combo.Path
	}

	// Collect all unique overlay keys from both before and after
	allOverlayKeys := make(map[string]bool)
	for _, combo := range beforeCombos {
		allOverlayKeys[combo.OverlayKey] = true
	}
	for _, combo := range afterCombos {
		allOverlayKeys[combo.OverlayKey] = true
	}

	results := make(map[string]models.BuildEnvManifestResult)
	overlayKeys := make([]string, 0, len(allOverlayKeys)) // Preserve order

	// Build a consistent order - use beforeCombos order first, then add any after-only keys
	for _, beforeCombo := range beforeCombos {
		overlayKeys = append(overlayKeys, beforeCombo.OverlayKey)
	}
	for _, afterCombo := range afterCombos {
		if _, exists := beforePathMap[afterCombo.OverlayKey]; !exists {
			overlayKeys = append(overlayKeys, afterCombo.OverlayKey)
		}
	}

	for _, overlayKey := range overlayKeys {
		comboCtx, comboSpan := trace.StartSpan(ctx, fmt.Sprintf("BuildManifests.%s", overlayKey))

		beforePath := beforePathMap[overlayKey]
		afterPath := afterPathMap[overlayKey]

		// Build before manifest
		logger.WithField("overlayKey", overlayKey).WithField("beforePath", beforePath).Info("Building before manifest...")
		beforeManifest, beforeErr := r.Builder.BuildAtFullPath(comboCtx, beforePath)
		beforeNotFound := beforeErr != nil && errors.Is(beforeErr, kustomize.ErrOverlayNotFound)
		if beforeErr != nil && !beforeNotFound {
			comboSpan.End()
			return nil, beforeErr
		}

		// Build after manifest
		logger.WithField("overlayKey", overlayKey).WithField("afterPath", afterPath).Info("Building after manifest...")
		afterManifest, afterErr := r.Builder.BuildAtFullPath(comboCtx, afterPath)
		afterNotFound := afterErr != nil && errors.Is(afterErr, kustomize.ErrOverlayNotFound)
		if afterErr != nil && !afterNotFound {
			comboSpan.End()
			return nil, afterErr
		}

		// Handle different scenarios
		if beforeNotFound && afterNotFound {
			// Both not found: skip this overlay entirely
			logger.WithField("overlayKey", overlayKey).Warn("Overlay not found in both before and after paths, marking as skipped")
			results[overlayKey] = models.BuildEnvManifestResult{
				OverlayKey:    overlayKey,
				Environment:   overlayKey,
				FullBuildPath: afterPath,
				Skipped:       true,
				SkipReason:    "overlay not found in both before and after paths",
			}
			comboSpan.End()
			continue
		}

		// At least one side exists, proceed with build result
		if beforeNotFound {
			logger.WithField("overlayKey", overlayKey).Info("Overlay not found in before path, treating as empty (new overlay)")
			beforeManifest = []byte{} // Treat as empty manifest
		}
		if afterNotFound {
			logger.WithField("overlayKey", overlayKey).Info("Overlay not found in after path, treating as empty (deletion)")
			afterManifest = []byte{} // Treat as empty manifest
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
		OverlayKeys:      overlayKeys, // Preserve the order
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

		// Use the ordered OverlayKeys from BuildManifestResult to preserve ordering
		reportData.OverlayKeys = rs.OverlayKeys
		reportData.Environments = rs.OverlayKeys // For backward compat in templates

		// Add parsed build values (use BeforePathBuilder or AfterPathBuilder)
		if r.Options.BeforePathBuilder != nil {
			reportData.ParsedKustomizeBuildValues = r.Options.BeforePathBuilder.Variables
		} else if r.Options.AfterPathBuilder != nil {
			reportData.ParsedKustomizeBuildValues = r.Options.AfterPathBuilder.Variables
		}
	} else if r.Options.UseDynamicPaths() {
		// Shared dynamic mode
		reportData.KustomizeBuildPath = r.Options.KustomizeBuildPath
		reportData.KustomizeBuildValues = r.Options.KustomizeBuildValues

		// Use the ordered OverlayKeys from BuildManifestResult to preserve ordering
		reportData.OverlayKeys = rs.OverlayKeys
		reportData.Environments = rs.OverlayKeys // For backward compat in templates

		// Add parsed build values if PathBuilder is available
		if r.Options.PathBuilder != nil {
			reportData.ParsedKustomizeBuildValues = r.Options.PathBuilder.Variables
		}
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
