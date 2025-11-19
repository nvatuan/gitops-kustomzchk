package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/diff"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/github"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/kustomize"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/models"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/policy"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/template"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/trace"
)

const (
	// GitHub Comment body length limit is 65536 characters, the default Markdown comment is about 2k characters.
	// 10k is a reasonable limit for the diff content, as it is arguably humanly impossible to read a diff that is longer.
	GH_COMMENT_MAX_DIFF_LENGTH = 10_000
)

var (
	githubCommentMaxDiffLength = GH_COMMENT_MAX_DIFF_LENGTH
)

type RunnerGitHub struct {
	RunnerBase

	options  *Options
	ghclient *github.Client

	runId    int
	prInfo   *models.PullRequest
	comments []*models.Comment
}

func NewRunnerGitHub(
	ctx context.Context,
	options *Options,
	ghclient *github.Client,
	builder *kustomize.Builder,
	differ *diff.Differ,
	evaluator *policy.PolicyEvaluator,
	renderer *template.Renderer,
) (*RunnerGitHub, error) {
	if ghclient == nil {
		return nil, fmt.Errorf("GitHub client is not initialized")
	}
	baseRunner, err := NewRunnerBase(ctx, options, builder, differ, evaluator, renderer)
	if err != nil {
		return nil, err
	}
	runner := &RunnerGitHub{
		RunnerBase: *baseRunner,
		ghclient:   ghclient,
		options:    options,
	}
	return runner, nil
}

func (r *RunnerGitHub) Initialize() error {
	lg := logger.WithField("func", "RunnerGitHub.Initialize()")
	lg.Info("Initializing runner: starting...")

	if err := r.fetchAndSetPullRequestInfo(); err != nil {
		return fmt.Errorf("failed to fetch pull request info: %w", err)
	}
	r.runId = 0
	runIdStr := os.Getenv("GITHUB_RUN_ID")
	if runIdStr != "" {
		if _, err := fmt.Sscanf(runIdStr, "%d", &r.runId); err != nil {
			lg.WithField("GITHUB_RUN_ID", runIdStr).WithField("error", err).Warn("GITHUB_RUN_ID env was set but failed to parse into int. Will not have artifact URLs in the diffs.")
		}
	} else {
		lg.Warn("GITHUB_RUN_ID env was not set. Artifact Uploading will not have artifact URLs in the comment.")
	}

	if maxDiffLengthStr := os.Getenv("GITHUB_COMMENT_MAX_DIFF_LENGTH"); maxDiffLengthStr != "" {
		if _, err := fmt.Sscanf(maxDiffLengthStr, "%d", &githubCommentMaxDiffLength); err != nil {
			lg.WithField("GITHUB_COMMENT_MAX_DIFF_LENGTH", maxDiffLengthStr).WithField("error", err).Warn("GITHUB_COMMENT_MAX_DIFF_LENGTH env was set but failed to parse into int. Will use default value of 10,000.")
			githubCommentMaxDiffLength = GH_COMMENT_MAX_DIFF_LENGTH
		}
	}
	lg.Info("Initializing runner: done.")
	return r.RunnerBase.Initialize()
}

// Fetch and set pull request data into struct from GitHub
func (r *RunnerGitHub) fetchAndSetPullRequestInfo() error {
	// Create channels for parallel execution
	type prResult struct {
		pr  *models.PullRequest
		err error
	}
	type commentsResult struct {
		comments []*models.Comment
		err      error
	}

	prChan := make(chan prResult, 1)
	commentsChan := make(chan commentsResult, 1)

	// Fetch PR info in parallel
	go func() {
		pr, err := r.ghclient.GetPR(r.Context, r.options.GhRepo, r.options.GhPrNumber)
		prChan <- prResult{pr: pr, err: err}
	}()

	// Fetch comments in parallel
	go func() {
		comments, err := r.ghclient.GetComments(r.Context, r.options.GhRepo, r.options.GhPrNumber)
		commentsChan <- commentsResult{comments: comments, err: err}
	}()

	// Wait for both results
	select {
	case prRes := <-prChan:
		if prRes.err != nil {
			return fmt.Errorf("failed to get PR info: %w", prRes.err)
		}
		r.prInfo = prRes.pr
	case <-r.Context.Done():
		return fmt.Errorf("PR fetch cancelled: %w", r.Context.Err())
	}

	select {
	case commentsRes := <-commentsChan:
		if commentsRes.err != nil {
			return fmt.Errorf("failed to get PR comments: %w", commentsRes.err)
		}
		r.comments = commentsRes.comments
	case <-r.Context.Done():
		return fmt.Errorf("comments fetch cancelled: %w", r.Context.Err())
	}

	return nil
}

func (r *RunnerGitHub) BuildManifests(beforePath, afterPath string) (*models.BuildManifestResult, error) {
	return r.RunnerBase.BuildManifests(beforePath, afterPath)
}

func (r *RunnerGitHub) DiffManifests(result *models.BuildManifestResult) (map[string]models.EnvironmentDiff, error) {
	// First, get the base diff results
	diffs, err := r.RunnerBase.DiffManifests(result)
	if err != nil {
		return nil, err
	}

	for env, envDiff := range diffs {
		if len(envDiff.Content) > githubCommentMaxDiffLength {
			logger.WithFields(map[string]interface{}{
				"env":        env,
				"diffLength": len(envDiff.Content),
				"maxLength":  githubCommentMaxDiffLength,
			}).Info("Diff is too long, uploading as artifact")

			// Create filename for this diff
			filename := fmt.Sprintf("diff-pr%d-%s-%s.txt", r.options.GhPrNumber, env, r.options.Service)

			// Save diff content to file
			outputDir := r.Options.OutputDir
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create output directory: %w", err)
			}

			filepath := filepath.Join(outputDir, filename)
			if err := os.WriteFile(filepath, []byte(envDiff.Content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write diff file: %w", err)
			}

			// Upload file as artifact and get URL
			artifactURL, err := github.GetWorkflowRunUrl(r.options.GhRepo, r.runId)
			if err != nil {
				logger.WithField("error", err).Error("Failed to get workflow run URL, leaving content as text")
				artifactURL = ""
			}

			// Update the diff result to point to the artifact URL
			envDiff.ContentGHFilePath = &filepath
			envDiff.ContentType = models.DiffContentTypeGHArtifact
			envDiff.Content = artifactURL
			diffs[env] = envDiff

			logger.WithFields(map[string]interface{}{
				"env":         env,
				"filename":    filename,
				"artifactURL": artifactURL,
			}).Info("Diff uploaded as artifact successfully")
		}
	}

	return diffs, nil
}

func (r *RunnerGitHub) Process() error {
	ctx, span := trace.StartSpan(r.Context, "Process")
	defer span.End()

	logger.Info("Process: starting...")

	logger.WithField("repo", r.options.GhRepo).WithField("branch", r.prInfo.BaseRef).Debug("Process: Calling CheckoutAtPath for base commit")
	_, checkoutBaseSpan := trace.StartSpan(ctx, "GitCheckout.Base")
	beforePathToSparseCheckout := filepath.Join(r.options.ManifestsPath, r.options.Service)
	checkedOutBeforePath, err := r.ghclient.CheckoutAtPath(
		r.Context, r.options.GhRepo, r.prInfo.BaseRef, beforePathToSparseCheckout, string(r.options.GitCheckoutStrategy))
	if err != nil {
		checkoutBaseSpan.End()
		return fmt.Errorf("failed to checkout base commit: %w", err)
	}
	checkoutBaseSpan.End()
	defer func() {
		_ = os.RemoveAll(checkedOutBeforePath)
	}()
	beforePath := filepath.Join(checkedOutBeforePath, r.options.ManifestsPath, r.options.Service)

	logger.WithField("repo", r.options.GhRepo).WithField("headRef", r.prInfo.HeadRef).Info("Checking out manifests")
	_, checkoutHeadSpan := trace.StartSpan(ctx, "GitCheckout.Head")

	afterPathToSparseCheckout := filepath.Join(r.options.ManifestsPath, r.options.Service)
	checkedOutAfterPath, err := r.ghclient.CheckoutAtPath(
		r.Context, r.options.GhRepo, r.prInfo.HeadRef, afterPathToSparseCheckout, string(r.options.GitCheckoutStrategy))
	if err != nil {
		checkoutHeadSpan.End()
		return fmt.Errorf("failed to checkout head commit: %w", err)
	}
	checkoutHeadSpan.End()
	defer func() {
		_ = os.RemoveAll(checkedOutAfterPath)
	}()
	afterPath := filepath.Join(checkedOutAfterPath, r.options.ManifestsPath, r.options.Service)

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

	ghComments, err := r.ghclient.GetComments(r.Context, r.options.GhRepo, r.options.GhPrNumber)
	if err != nil {
		return fmt.Errorf("failed to get comments: %w", err)
	}
	ghCommentStrings := make([]string, len(ghComments))
	for i, comment := range ghComments {
		ghCommentStrings[i] = comment.Body
	}

	_, evalSpan := trace.StartSpan(ctx, "EvaluatePolicies")
	policyEval, err := r.Evaluator.GeneratePolicyEvalResultForManifests(ctx, *rs, ghCommentStrings)
	if err != nil {
		evalSpan.End()
		return err
	}
	evalSpan.End()
	logger.WithField("results", policyEval).Debug("Evaluated Policies")

	reportData := models.ReportData{
		Service:          r.Options.Service,
		Timestamp:        time.Now(),
		BaseCommit:       r.prInfo.BaseSHA,
		HeadCommit:       r.prInfo.HeadSHA,
		Environments:     r.Options.Environments,
		ManifestChanges:  diffs,
		PolicyEvaluation: *policyEval,
	}

	if err := r.Output(&reportData); err != nil {
		return err
	}
	return nil
}

func (r *RunnerGitHub) Output(data *models.ReportData) error {
	_, span := trace.StartSpan(r.Context, "Output")
	defer span.End()

	logger.Info("Output: starting...")
	if err := r.outputReportJson(data); err != nil {
		return err
	}
	if err := r.outputGitHubComment(data); err != nil {
		return err
	}
	logger.Info("Output: done.")
	return nil
}

// Exporting report json file to output directory if enabled
func (r *RunnerGitHub) outputReportJson(data *models.ReportData) error {
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

// Post comment to GitHub PR
func (r *RunnerGitHub) outputGitHubComment(data *models.ReportData) error {
	logger.Info("OutputGitHubComment: starting...")

	// Render the markdown using templates
	renderedMarkdown, err := r.Renderer.RenderWithTemplates(r.Options.TemplatesPath, data)
	if err != nil {
		logger.WithField("error", err).Error("Failed to render markdown template")
		return err
	}
	logger.WithField("renderedMarkdown", renderedMarkdown).Debug("Rendered markdown")

	// Add the comment marker and replace the service token
	commentSignature := strings.ReplaceAll(template.ToolCommentSignature, template.ToolCommentServiceToken, r.Options.Service)
	finalComment := commentSignature + "\n\n" + renderedMarkdown

	// Check if there's an existing comment from this tool for this specific service
	// We search for the comment signature to find the right comment
	existingComment, err := r.ghclient.FindToolComment(r.Context, r.options.GhRepo, r.options.GhPrNumber, commentSignature)
	if err != nil {
		logger.WithField("error", err).Warn("Failed to find existing comment, will create new one")
	}

	if existingComment != nil {
		// Update existing comment
		if err := r.ghclient.UpdateComment(r.Context, r.options.GhRepo, existingComment.ID, finalComment); err != nil {
			logger.WithField("error", err).Error("Failed to update existing comment")
			return err
		}
		logger.Info("Updated existing GitHub comment")
	} else {
		// Create new comment
		if _, err := r.ghclient.CreateComment(r.Context, r.options.GhRepo, r.options.GhPrNumber, finalComment); err != nil {
			logger.WithField("error", err).Error("Failed to create new comment")
			return err
		}
		logger.Info("Created new GitHub comment")
	}

	return nil
}
