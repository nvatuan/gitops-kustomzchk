package main

import (
	"context"
	"fmt"

	"github.com/gh-nvat/gitops-kustomzchk/src/internal/runner"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/diff"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/github"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/kustomize"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/policy"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/template"
	"github.com/gh-nvat/gitops-kustomzchk/src/pkg/trace"
	log "github.com/sirupsen/logrus"
)

var logger *log.Entry = log.New().WithFields(log.Fields{
	"package": "run",
})

const (
	RUN_MODE_GITHUB = "github"
	RUN_MODE_LOCAL  = "local"
)

// Initialize creates and initializes the appropriate runner
func createRunner(ctx context.Context, opts *runner.Options) (runner.RunnerInterface, error) {
	logger.WithField("opts", opts).Debug("Creating runner..")

	builder := kustomize.NewBuilderWithOptions(opts.FailOnOverlayNotFound)
	differ := diff.NewDiffer()
	evaluator := policy.NewPolicyEvaluator(opts.PoliciesPath)
	renderer := template.NewRenderer()

	switch opts.RunMode {
	case RUN_MODE_GITHUB:
		ghClient, err := github.NewClient()
		if err != nil {
			return nil, fmt.Errorf("GitHub authentication failed: %w", err)
		}
		runner, err := runner.NewRunnerGitHub(
			ctx, opts, ghClient, builder, differ, evaluator, renderer)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub runner: %w", err)
		}
		return runner, nil
	case RUN_MODE_LOCAL:
		runner, err := runner.NewRunnerLocal(
			ctx, opts, builder, differ, evaluator, renderer,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Local runner: %w", err)
		}
		return runner, nil
	default:
		return nil, fmt.Errorf("invalid run mode: %s", opts.RunMode)
	}
}

func initialize(ctx context.Context, opts *runner.Options) (runner.RunnerInterface, error) {
	runner, err := createRunner(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}
	if err := runner.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize runner: %w", err)
	}
	return runner, nil
}

func run(ctx context.Context, opts *runner.Options) error {
	logger.WithField("opts", opts).Info("Running..")
	if opts.Debug {
		log.SetLevel(log.DebugLevel)
	}

	// Initialize tracer
	shutdown, err := trace.InitTracer("gitops-kustomz", opts.EnableExportPerformanceReport, opts.OutputDir)
	if err != nil {
		return fmt.Errorf("failed to initialize tracer: %w", err)
	}
	defer shutdown()

	// Validate options
	if err := validateOptions(opts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Initialize runner
	appRunner, err := initialize(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	err = appRunner.Process()
	if err != nil {
		return fmt.Errorf("failed to process: %w", err)
	}

	return nil
}

func validateOptions(opts *runner.Options) error {
	// Validate run mode
	if opts.RunMode != "github" && opts.RunMode != "local" {
		return fmt.Errorf("run-mode must be 'github' or 'local', got: %s", opts.RunMode)
	}

	// Check which flag set is being used
	useDynamic := opts.KustomizeBuildPath != "" || opts.KustomizeBuildValues != ""
	useLegacy := opts.Service != "" || len(opts.Environments) > 0

	// Validate that one set of flags is used, not both
	if useDynamic && useLegacy {
		return fmt.Errorf("cannot mix legacy flags (--service, --environments) with new flags (--kustomize-build-path, --kustomize-build-values)")
	}

	// Validate that at least one set is provided
	if !useDynamic && !useLegacy {
		return fmt.Errorf("must provide either:\n  - New flags: --kustomize-build-path and --kustomize-build-values\n  - Legacy flags: --service and --environments")
	}

	// Validate dynamic path flags
	if useDynamic {
		if opts.KustomizeBuildPath == "" {
			return fmt.Errorf("--kustomize-build-path is required when using dynamic paths")
		}
		if opts.KustomizeBuildValues == "" {
			return fmt.Errorf("--kustomize-build-values is required when using dynamic paths")
		}
		// Initialize PathBuilder
		if err := opts.InitializePathBuilder(); err != nil {
			return fmt.Errorf("invalid kustomize build configuration: %w", err)
		}
	}

	// Validate legacy flags
	if useLegacy {
		if opts.Service == "" {
			return fmt.Errorf("--service is required when using legacy flags")
		}
		if len(opts.Environments) == 0 {
			return fmt.Errorf("--environments is required when using legacy flags")
		}
	}

	// Validate mode-specific options
	if opts.RunMode == "local" {
		if opts.LcBeforeManifestsPath == "" || opts.LcAfterManifestsPath == "" {
			return fmt.Errorf("local mode requires --lc-before-manifests-path and --lc-after-manifests-path")
		}
	} else {
		// GitHub mode
		if opts.GhRepo == "" {
			return fmt.Errorf("github mode requires --gh-repo")
		}
		if opts.GhPrNumber == 0 {
			return fmt.Errorf("github mode requires --gh-pr-number")
		}
		// Validate git checkout strategy
		if opts.GitCheckoutStrategy == "" {
			opts.GitCheckoutStrategy = runner.GitCheckoutStrategySparse // default
		}
		if opts.GitCheckoutStrategy != runner.GitCheckoutStrategySparse &&
			opts.GitCheckoutStrategy != runner.GitCheckoutStrategyShallow {
			return fmt.Errorf("git-checkout-strategy must be 'sparse' or 'shallow', got: %s", opts.GitCheckoutStrategy)
		}
	}

	return nil
}
