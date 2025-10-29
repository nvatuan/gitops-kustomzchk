# gitops-kustomz

<p align="center">
  <img src="docs/gitops-kustomz-rec.png" alt="gitops-kustomz logo" width="200"/>
</p>

GitOps policy enforcement tool for Kubernetes manifests managed with Kustomize.


## Overview

`gitops-kustomz` is designed to run in GitHub Actions CI on Pull Requests. It analyzes Kubernetes manifest changes managed with Kustomize, enforces OPA policies, and provides detailed feedback via PR comments.

## Features

- 🔍 **Kustomize Build & Diff**: Builds manifests from base and head branches, generates clear diffs
- 📋 **Policy Enforcement**: Evaluates OPA policies with configurable enforcement levels (RECOMMEND/WARNING/BLOCK)
- 💬 **GitHub Integration**: Posts detailed policy reports and diffs as PR comments
- 📎 **Smart Diff Handling**: Automatically uploads large diffs (>10k chars) as GitHub artifacts with links in PR comments
- 🔗 **Policy Documentation**: Add external links to policies for easy access to documentation
- 📊 **Enhanced Policy Matrix**: View all policies with enforcement levels in a comprehensive table
- ⚡ **Fast**: Parallel policy evaluation with goroutines, <2s build time target
- 🧪 **Local Testing**: Test policies locally without GitHub PR
- 📈 **Performance Tracing**: Optional performance reports with detailed timing for each step
- 🧹 **Clean Diffs**: Kustomize warnings filtered out from diff output

## Quick Start

### GitHub Actions (Recommended)

Copy one of the sample workflows to your GitOps repository:

```bash
# Copy workflow to your repo
cp sample/github-actions/gitops-policy-check-multi-env.yml \
   .github/workflows/gitops-policy-check.yml
```

See [sample/github-actions/README.md](./sample/github-actions/README.md) for detailed setup instructions.

### CLI Usage

```bash
# Run on a PR (GitHub mode)
gitops-kustomz \
  --run-mode github \
  --gh-repo owner/repo \
  --gh-pr-number 123 \
  --service my-app \
  --environments stg,prod \
  --manifests-path ./services \
  --policies-path ./policies \
  --enable-export-performance-report true  # Optional: export performance metrics

# Local testing
gitops-kustomz \
  --run-mode local \
  --service my-app \
  --environments stg,prod \
  --lc-before-manifests-path ./before/services \
  --lc-after-manifests-path ./after/services \
  --policies-path ./policies \
  --output-dir ./output \
  --enable-export-report true \
  --enable-export-performance-report true  # Optional: export performance metrics
```

## 📁 Project Structure

```
.
├── src/
│   ├── cmd/gitops-kustomz/    # CLI entry point
│   ├── pkg/                   # Core packages
│   │   ├── diff/              # Manifest diffing
│   │   ├── github/            # GitHub API client & sparse checkout
│   │   ├── kustomize/         # Kustomize builder
│   │   ├── models/            # Data models for reports & configs
│   │   ├── policy/            # Policy evaluation (OPA/Conftest)
│   │   ├── template/          # Markdown templating
│   │   └── trace/             # Performance tracing with OpenTelemetry
│   ├── internal/
│   │   └── runner/            # GitHub & Local runners
│   └── templates/             # Default markdown templates
├── sample/                    # Example policies & manifests
│   ├── github-actions/        # Sample workflows
│   ├── k8s-manifests/         # Sample Kubernetes manifests
│   └── policies/              # Sample OPA policies
├── test/                      # Test data & System Integration Tests
├── go.mod                     # Go module definition
└── Makefile                   # Build automation
```

## Template Customization

The tool supports custom markdown templates for GitHub comments. Templates use Go's `text/template` syntax with rich data structures.

### Quick Template Examples

```go
// Service and environment info
{{.Service}} - {{range .Environments}}{{.}} {{end}}

// Timestamp formatting
{{.Timestamp.Format "2006-01-02 15:04:05 UTC"}}

// Conditional rendering
{{if gt .MultiEnvPolicyReport.Summary.stg.FailedPolicies 0}}
  ⚠️ Staging has failed policies
{{end}}

// Policy status matrix
{{range .MultiEnvPolicyReport.Policies}}
  {{.Name}}: {{.Level}}
{{end}}
```

See [docs/TEMPLATE_VARIABLES.md](./docs/TEMPLATE_VARIABLES.md) for complete reference.

## Policy Configuration

Policies are defined in `compliance-config.yaml` with support for:

- **Enforcement Levels**: BLOCKING, WARNING, RECOMMEND
- **Time-based Enforcement**: Policies can change levels over time
- **Override Support**: Allow policy bypass via PR comments
- **External Links**: Link to policy documentation for easy reference

### Example Policy Configuration

```yaml
policies:
  service-high-availability:
    name: Service High Availability
    description: Ensures deployments meet HA criteria
    type: opa
    filePath: ha.rego
    externalLink: https://docs.example.com/policies/high-availability  # Optional
    
    enforcement:
      inEffectAfter: 2025-10-01T00:00:00Z
      isWarningAfter: 2025-11-01T00:00:00Z
      isBlockingAfter: 2025-12-01T00:00:00Z
      override:
        comment: "/override-ha"
```

### Policy Report Features

- **Policy Evaluation Matrix**: Comprehensive table showing all policies with enforcement levels
- **Detailed Failure Reports**: Organized by enforcement level (BLOCKING, WARNING, RECOMMEND)
- **External Links**: Clickable policy names in the matrix that link to documentation
- **Pass/Fail Status**: Clear indicators for each environment

## Recent Updates

### v0.1.0+ Features

- **Smart Diff Artifacts** (#3): Diffs >10k chars automatically uploaded as GitHub artifacts
- **System Integration Tests** (#2): Automated testing for local mode with baseline comparison
- **Clean Diff Output** (#4): Kustomize stderr warnings no longer pollute diff output
- **Enhanced Policy Matrix** (#5): Added Level column and external link support
- **Policy Detail Improvements**: Only failed policies shown in details section
- **Performance Tracing**: Optional OpenTelemetry-based performance reports

## Documentation

- [sample/github-actions/README.md](./sample/github-actions/README.md) - **GitHub Actions setup guide**
- [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) - High-level architecture and use cases
- [docs/DESIGN.md](./docs/DESIGN.md) - Detailed design and implementation specs
- [docs/TEMPLATE_VARIABLES.md](./docs/TEMPLATE_VARIABLES.md) - **Template variables and functions reference**
- [LOCAL_TESTING.md](./LOCAL_TESTING.md) - Local testing guide

## Requirements

- Go 1.22+
- `kustomize` binary in PATH
- `conftest` binary in PATH (for OPA policy evaluation)
- GitHub token with PR comment permissions (for CI mode)

## Environment Variables

### GitHub Mode
- `GH_TOKEN` or `GITHUB_TOKEN` - GitHub personal access token with PR comment permissions (required)
- `GITHUB_RUN_ID` or `GH_RUN_ID` - GitHub Actions run ID (auto-set by GitHub Actions, used for artifact URLs)

### Optional Configuration
- `LOGLEVEL` - Log level for the application (default: `info`, options: `debug`, `info`, `warn`, `error`)
- `DEBUG` - Enable debug mode (set to `1` or `true`)
- `GH_MAX_COMMENT_LENGTH` - Maximum length for inline diffs in PR comments (default: `10000` characters). Diffs exceeding this limit will be uploaded as artifacts.

## Installation

```bash
go install github.com/gh-nvat/gitops-kustomz@latest
```

## Development

```bash
# Clone the repo
git clone https://github.com/gh-nvat/gitops-kustomz.git
cd gitops-kustomz

# Build
make build

# Run tests
make test

# Run linter
make lint

# Local testing mode
make run-local

# System Integration Test (compares output with baseline)
make sit-test-local
```

## License

MIT


