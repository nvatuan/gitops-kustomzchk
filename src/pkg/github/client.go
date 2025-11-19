package github

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/models"
	"github.com/google/go-github/v66/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

var logger = log.WithField("package", "github")

// GitHubClient defines the interface for GitHub API operations
type GitHubClient interface {
	// GetPR retrieves pull request information
	GetPR(ctx context.Context, repo string, number int) (*models.PullRequest, error)
	// CreateComment creates a new comment on a pull request
	CreateComment(ctx context.Context, repo string, number int, body string) (*models.Comment, error)
	// UpdateComment updates an existing comment
	UpdateComment(ctx context.Context, repo string, commentID int64, body string) error
	// GetComments retrieves all comments for a pull request
	GetComments(ctx context.Context, repo string, number int) ([]*models.Comment, error)
	// FindToolComment finds an existing tool-generated comment containing the search string
	FindToolComment(ctx context.Context, repo string, prNumber int, searchString string) (*models.Comment, error)
	// CheckoutAtPath clones and checks out specific ref at path with the specified strategy
	CheckoutAtPath(ctx context.Context, cloneURL, ref, path, strategy string) (string, error)
}

// Client handles GitHub API interactions using go-github
type Client struct {
	client *github.Client
}

// Ensure Client implements GitHubClient
var _ GitHubClient = (*Client)(nil)

// NewClient creates a new GitHub client
func NewClient() (*Client, error) {
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("GitHub token not found. Set GH_TOKEN or GITHUB_TOKEN environment variable")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &Client{
		client: client,
	}, nil
}

// GetPR retrieves pull request information
func (c *Client) GetPR(ctx context.Context, repo string, number int) (*models.PullRequest, error) {
	owner, repo, err := ParseOwnerRepo(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR: %w", err)
	}

	return &models.PullRequest{
		Number:  pr.GetNumber(),
		BaseRef: pr.GetBase().GetRef(),
		BaseSHA: pr.GetBase().GetSHA(),
		HeadRef: pr.GetHead().GetRef(),
		HeadSHA: pr.GetHead().GetSHA(),
	}, nil
}

// CreateComment creates a new comment on a pull request
func (c *Client) CreateComment(ctx context.Context, repo string, number int, body string) (*models.Comment, error) {
	owner, repo, err := ParseOwnerRepo(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}
	comment := &github.IssueComment{
		Body: github.String(body),
	}

	created, _, err := c.client.Issues.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %w", err)
	}

	return &models.Comment{
		ID:   created.GetID(),
		Body: created.GetBody(),
	}, nil
}

// UpdateComment updates an existing comment
func (c *Client) UpdateComment(ctx context.Context, repo string, commentID int64, body string) error {
	owner, repo, err := ParseOwnerRepo(repo)
	if err != nil {
		return fmt.Errorf("failed to parse repository: %w", err)
	}
	comment := &github.IssueComment{
		Body: github.String(body),
	}

	commentRes, res, err := c.client.Issues.EditComment(ctx, owner, repo, commentID, comment)
	log.WithField("comment", commentRes).WithField("response", res).Debug("Updated comment")
	if err != nil {
		return fmt.Errorf("failed to update comment: %w", err)
	}

	return nil
}

// GetComments retrieves all comments for a pull request
// Current limitation it will only fetch first 200 comments, hopefully it contains override messages..
func (c *Client) GetComments(ctx context.Context, repo string, prNumber int) ([]*models.Comment, error) {
	owner, repo, err := ParseOwnerRepo(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 200},
	}

	var allComments []*models.Comment
	for {
		comments, resp, err := c.client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get comments: %w", err)
		}

		for _, c := range comments {
			allComments = append(allComments, &models.Comment{
				ID:   c.GetID(),
				Body: c.GetBody(),
			})
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// FindToolComment finds an existing tool-generated comment containing the search string
// If multiple comments with the same marker exist, returns the first one found
func (c *Client) FindToolComment(ctx context.Context, repo string, prNumber int, searchString string) (*models.Comment, error) {
	comments, err := c.GetComments(ctx, repo, prNumber)
	if err != nil {
		return nil, err
	}

	for _, comment := range comments {
		if strings.Contains(comment.Body, searchString) {
			return comment, nil
		}
	}

	return nil, nil // Returns nil if not found
}

// CheckoutAtPath clones and checks out specific ref at path with the specified strategy
// strategy: "sparse" (scoped to path) or "shallow" (all files, depth 1)
// returns the directory containing the checked out files
// For sparse strategy, it does the following commands:
// 1. git clone --filter=blob:none --depth 1 --no-checkout --single-branch -b branch cloneURL directory
// 2. git sparse-checkout set --no-cone path
// 3. git checkout branch
// 4. return directory
// For shallow strategy, it does:
// 1. git clone --depth 1 --single-branch -b branch cloneURL directory
// 2. return directory
func (c *Client) CheckoutAtPath(ctx context.Context, repo, branch, path, strategy string) (string, error) {
	logger.WithField("repo", repo).WithField("branch", branch).WithField("path", path).WithField("strategy", strategy).Info("CheckoutAtPath()")

	// create /tmp at pwd if not exists
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get pwd: %w", err)
	}
	tmpdir := filepath.Join(pwd, "tmp")
	if err := os.MkdirAll(tmpdir, 0755); err != nil {
		return "", fmt.Errorf("failed to create tmpdir at %s: %w", tmpdir, err)
	}

	chkoutName := strings.ReplaceAll(branch, "/", "_")
	checkoutDir := fmt.Sprintf("chk-%s-%d", chkoutName, time.Now().Unix())
	cloneURL, err := GetHTTPSCloneURLForRepo(repo)
	if err != nil {
		return "", fmt.Errorf("failed to get clone URL: %w", err)
	}

	// Use GitHub token for authentication
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		// Use x-access-token as username with token as password
		cloneURL = strings.Replace(cloneURL, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}

	if strategy == "shallow" {
		// Shallow checkout: all files, depth 1
		logger.WithField("tmpdir", tmpdir).WithField("checkoutDir", checkoutDir).Debug("Shallow cloning (all files)...")
		cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--single-branch", "-b", branch, cloneURL, checkoutDir)
		logger.WithField("cloneCmd", cloneCmd.String()).Debug("Showing clone command")
		cloneCmd.Dir = tmpdir
		var cloneStdout, cloneStderr bytes.Buffer
		cloneCmd.Stdout = &cloneStdout
		cloneCmd.Stderr = &cloneStderr
		if err := cloneCmd.Run(); err != nil {
			logger.WithField("stdout", cloneStdout.String()).WithField("stderr", cloneStderr.String()).Error("Shallow clone failed")
			return "", fmt.Errorf("failed to shallow clone: %w\nStdout: %s\nStderr: %s", err, cloneStdout.String(), cloneStderr.String())
		}
		logger.WithField("stdout", cloneStdout.String()).WithField("stderr", cloneStderr.String()).Debug("Shallow clone succeeded")

		absPath, err := filepath.Abs(filepath.Join(tmpdir, checkoutDir))
		logger.WithField("checkoutDir", checkoutDir).WithField("absPath", absPath).Debug("Absolute path...")
		if err != nil {
			_ = os.RemoveAll(filepath.Join(tmpdir, checkoutDir))
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
		return absPath, nil
	}

	// Sparse checkout (default): scoped to path
	// 1. git clone --filter=blob:none --depth 1 --no-checkout --single-branch -b branch cloneURL directory
	logger.WithField("tmpdir", tmpdir).WithField("checkoutDir", checkoutDir).Debug("Sparse cloning...")
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--filter=blob:none", "--depth", "1", "--no-checkout", "--single-branch", "-b", branch, cloneURL, checkoutDir)
	logger.WithField("cloneCmd", cloneCmd.String()).Debug("Showing clone command")
	cloneCmd.Dir = tmpdir
	var cloneStdout, cloneStderr bytes.Buffer
	cloneCmd.Stdout = &cloneStdout
	cloneCmd.Stderr = &cloneStderr
	if err := cloneCmd.Run(); err != nil {
		logger.WithField("stdout", cloneStdout.String()).WithField("stderr", cloneStderr.String()).Error("Clone failed")
		return "", fmt.Errorf("failed to clone: %w\nStdout: %s\nStderr: %s", err, cloneStdout.String(), cloneStderr.String())
	}
	logger.WithField("stdout", cloneStdout.String()).WithField("stderr", cloneStderr.String()).Debug("Clone succeeded")

	// 2. git sparse-checkout set --no-cone path
	logger.WithField("tmpdir", tmpdir).WithField("checkoutDir", checkoutDir).Debug("Set path sparse-checkout...")
	sparseCmd := exec.CommandContext(ctx, "git", "sparse-checkout", "set", "--no-cone", path)
	sparseCmd.Dir = filepath.Join(tmpdir, checkoutDir)
	logger.WithField("sparseCmd", sparseCmd.String()).Debug("Showing sparse-checkout command")
	var sparseStdout, sparseStderr bytes.Buffer
	sparseCmd.Stdout = &sparseStdout
	sparseCmd.Stderr = &sparseStderr
	if err := sparseCmd.Run(); err != nil {
		logger.WithField("stdout", sparseStdout.String()).WithField("stderr", sparseStderr.String()).Error("Sparse checkout set failed")
		_ = os.RemoveAll(filepath.Join(tmpdir, checkoutDir))
		return "", fmt.Errorf("failed to set sparse checkout: %w\nStdout: %s\nStderr: %s", err, sparseStdout.String(), sparseStderr.String())
	}
	logger.WithField("stdout", sparseStdout.String()).WithField("stderr", sparseStderr.String()).Debug("Sparse checkout set succeeded")

	// 3. git checkout branch
	logger.WithField("tmpdir", tmpdir).WithField("branch", branch).WithField("checkoutDir", checkoutDir).Debug("Check out branch...")
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", branch)
	checkoutCmd.Dir = filepath.Join(tmpdir, checkoutDir)
	logger.WithField("checkoutCmd", checkoutCmd.String()).Debug("Showing checkout command")
	var checkoutStdout, checkoutStderr bytes.Buffer
	checkoutCmd.Stdout = &checkoutStdout
	checkoutCmd.Stderr = &checkoutStderr
	if err := checkoutCmd.Run(); err != nil {
		logger.WithField("stdout", checkoutStdout.String()).WithField("stderr", checkoutStderr.String()).Error("Checkout failed")
		_ = os.RemoveAll(filepath.Join(tmpdir, checkoutDir))
		return "", fmt.Errorf("failed to checkout: %w\nStdout: %s\nStderr: %s", err, checkoutStdout.String(), checkoutStderr.String())
	}
	logger.WithField("stdout", checkoutStdout.String()).WithField("stderr", checkoutStderr.String()).Debug("Checkout succeeded")

	// 4. return directory
	absPath, err := filepath.Abs(filepath.Join(tmpdir, checkoutDir))
	logger.WithField("checkoutDir", checkoutDir).WithField("absPath", absPath).Debug("Absolute path...")
	if err != nil {
		_ = os.RemoveAll(filepath.Join(tmpdir, checkoutDir))
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// // this spots the case that --manfifest-path has a ./ prefix didn't work
	// list files with permissions in the following directory [pwd, tmpdir, checkoutDir]
	// logger.Info("DEBUGGING: LISTING FILES IN THE FOLLOWING DIRECTORIES [pwd, tmpdir, absPath]")
	// dirs := []string{pwd, tmpdir, absPath}
	// for _, dir := range dirs {
	// 	logger.WithField("dir", dir).Debug("Started list ls -la...")
	// 	lsCmd := exec.CommandContext(ctx, "ls", "-la", dir)
	// 	output, err := lsCmd.CombinedOutput()
	// 	if err != nil {
	// 		return "", fmt.Errorf("failed to list directory %s: %w\nOutput: %s", dir, err, string(output))
	// 	}
	// 	logger.WithField("dir", dir).WithField("output", string(output)).Debug("Listed directory...")
	// }

	return absPath, nil
}
