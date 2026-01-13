# Implementation Plan: Dynamic Kustomize Build Path Feature

## Overview
Replace hardcoded path structure (`<MANIFESTS_PATH>/<SERVICE>/environments/<ENVIRONMENT>`) with a flexible, variable-based system that supports arbitrary overlay structures.

## Current State Analysis

**Current (v0.4) hardcoded approach:**
- Path: `<manifests-path>/<service>/environments/<env>`
- Flags: `--manifests-path`, `--service`, `--environments`
- Hardcoded: `KUSTOMIZE_OVERLAY_DIR_NAME = "environments"`

**Limitations:**
- Can't handle nested overlays like `services/my-app/clusters/alpha/stg`
- Not flexible for different kustomize structures

## Proposed Design

### 1. New CLI Flags

**Remove (Breaking Change):**
- ~~`--manifests-path`~~
- ~~`--service`~~
- ~~`--environments`~~

**Add:**
- `--kustomize-build-path` (string): Template path with `$VARIABLE` placeholders
  - Example: `/home/me/k8s-manifests/services/$SERVICE/clusters/$CLUSTER/envs/$ENV`
- `--kustomize-build-values` (string): Semicolon-separated key-value assignments
  - Format: `KEY=value1,value2;KEY2=value3`
  - Example: `SERVICE=my-app;CLUSTER=alpha,beta;ENV=stg,prod`

### 2. Core Changes

#### A. New Package: `pkg/pathbuilder` (NEW)
```go
// PathBuilder interpolates variables into build path templates
type PathBuilder struct {
    Template  string              // e.g., "/path/$SERVICE/clusters/$CLUSTER/$ENV"
    Variables map[string][]string // e.g., {"SERVICE": ["my-app"], "CLUSTER": ["alpha","beta"], ...}
}

// Methods:
- ParseTemplate(template string) ([]string, error) // Extract $VARIABLE names
- ParseValues(valuesStr string) (map[string][]string, error) // Parse "KEY=v1,v2;..."
- Validate() error // Check all $VARs have values
- GenerateAllPaths() ([]PathCombination, error) // Cartesian product of all values
- InterpolatePath(values map[string]string) (string, error) // Single interpolation
- GetRelativePaths() ([]string, error) // Get all relative paths for sparse checkout

type PathCombination struct {
    Path      string            // Full interpolated path
    Values    map[string]string // Variable values used
    OverlayKey string           // Key for reports (e.g., "my-app/alpha/stg")
}
```

#### B. Update `pkg/kustomize/builder.go`
```go
// Remove hardcoded constants:
// - KUSTOMIZE_OVERLAY_DIR_NAME

// Modify Builder interface:
type KustomizeBuilder interface {
    Build(ctx context.Context, fullPath string) ([]byte, error) // No more "overlayName" param
    BuildToText(ctx context.Context, fullPath string) (string, error)
}

// Update implementation:
- Remove getBuildPath() logic
- buildAtPath() stays the same (already accepts full path)
- validateBuildPath() simplified: just check if kustomization.yaml exists at fullPath
```

#### C. Update `internal/runner/options.go`
```go
type Options struct {
    // New fields
    KustomizeBuildPath   string // Template with $VARs
    KustomizeBuildValues string // "KEY=v1,v2;KEY2=v3"
    
    // Computed internally (not CLI flags)
    pathBuilder *pathbuilder.PathBuilder // Parsed path builder
    
    // ... rest of existing fields
}
```

#### D. Update `cmd/gitops-kustomzchk/main.go`
```go
// Add new flags (REQUIRED)
cmd.Flags().StringVar(&opts.KustomizeBuildPath, "kustomize-build-path", "", 
    "Path template with $VARIABLES (e.g., '/path/$SERVICE/clusters/$CLUSTER/$ENV')")
cmd.Flags().StringVar(&opts.KustomizeBuildValues, "kustomize-build-values", "",
    "Variable values: 'KEY=v1,v2;KEY2=v3' (e.g., 'SERVICE=my-app;ENV=stg,prod')")

// Mark as required
_ = cmd.MarkFlagRequired("kustomize-build-path")
_ = cmd.MarkFlagRequired("kustomize-build-values")

// Remove old flags entirely: --service, --environments, --manifests-path
```

#### E. Update `internal/runner/base.go`
```go
func (r *RunnerBase) Initialize() error {
    // NEW: Initialize pathBuilder from options
    r.Options.pathBuilder = pathbuilder.NewPathBuilder(
        r.Options.KustomizeBuildPath,
        r.Options.KustomizeBuildValues,
    )
    if err := r.Options.pathBuilder.Validate(); err != nil {
        return fmt.Errorf("invalid path configuration: %w", err)
    }
    
    // ... rest
}

func (r *RunnerBase) BuildManifests() (*models.BuildManifestResult, error) {
    // CHANGED: Generate all path combinations
    pathCombos, err := r.Options.pathBuilder.GenerateAllPaths()
    if err != nil {
        return nil, err
    }
    
    results := make(map[string]models.BuildOverlayManifestResult)
    
    for _, combo := range pathCombos {
        // Build for "before" and "after" using combo.Path
        beforeFullPath := filepath.Join(beforeRoot, combo.Path)
        afterFullPath := filepath.Join(afterRoot, combo.Path)
        
        beforeManifest, err := r.Builder.Build(ctx, beforeFullPath)
        afterManifest, err := r.Builder.Build(ctx, afterFullPath)
        
        // Use combo.OverlayKey as the key
        results[combo.OverlayKey] = models.BuildOverlayManifestResult{
            OverlayKey:     combo.OverlayKey,
            BeforeManifest: beforeManifest,
            AfterManifest:  afterManifest,
        }
    }
    
    return &models.BuildManifestResult{OverlayManifestBuild: results}, nil
}
```

#### F. Update `internal/runner/github.go`
```go
func (r *RunnerGitHub) Process() error {
    // CHANGED: Checkout strategy selection based on combinations count
    pathCombos, _ := r.Options.pathBuilder.GenerateAllPaths()
    
    if len(pathCombos) < 4 {
        // Use sparse checkout for few combinations (better performance)
        // Checkout each specific path
        relativePaths := make([]string, len(pathCombos))
        for i, combo := range pathCombos {
            relativePaths[i] = combo.Path // or combo.RelativePath
        }
        
        // Checkout base with all specific paths
        checkedOutPath, err := r.ghclient.CheckoutAtPathWithMultiplePaths(
            r.Context, r.options.GhRepo, r.prInfo.BaseRef, relativePaths, "sparse")
    } else {
        // Use shallow checkout for many combinations (checkout everything)
        checkedOutPath, err := r.ghclient.CheckoutAtPath(
            r.Context, r.options.GhRepo, r.prInfo.BaseRef, ".", "shallow")
    }
    
    // Rest remains similar but uses pathBuilder.GenerateAllPaths()
}
```

### 3. Model Changes

#### A. `pkg/models/kustomize_result.go`
```go
type BuildOverlayManifestResult struct {
    OverlayKey     string // Key like "my-app/clusters/alpha/stg"
    FullBuildPath  string // NEW: Store the actual path used for building
    BeforeManifest []byte
    AfterManifest  []byte
    Skipped        bool
    SkipReason     string
}
```

#### B. `pkg/models/reportdata.go`
```go
type ReportData struct {
    // NEW: Store the build configuration for reference
    KustomizeBuildPath   string   `json:"kustomizeBuildPath"`
    KustomizeBuildValues string   `json:"kustomizeBuildValues"`
    
    Timestamp    time.Time `json:"timestamp"`
    BaseCommit   string    `json:"baseCommit"`
    HeadCommit   string    `json:"headCommit"`
    OverlayKeys  []string  `json:"overlayKeys"` // RENAMED from Environments

    // Key = OverlayKey
    ManifestChanges  map[string]OverlayDiff   `json:"manifestChanges"`
    PolicyEvaluation PolicyEvaluation         `json:"policyEvaluation"`
}

type OverlayDiff struct {
    LineCount        int     `json:"lineCount"`
    AddedLineCount   int     `json:"addedLineCount"`
    DeletedLineCount int     `json:"deletedLineCount"`
    ContentType      string  `json:"contentType"`
    Content          string  `json:"content"`
}
```

### 4. Template Changes

Update templates to use new field names:

**`src/templates/diff.md.tmpl`**
```markdown
{{range $overlayKey, $diff := .ManifestChanges}}
### [`{{$overlayKey}}`]: {{if gt $diff.LineCount 0}}...
```

**`src/templates/comment.md.tmpl`** & **`policy.md.tmpl`**
- Update to use `.OverlayKeys` instead of `.Environments`
- Iterate using `$overlayKey` instead of `$env`

### 5. Testing Strategy

#### A. Unit Tests (NEW)
- `pkg/pathbuilder/pathbuilder_test.go`
  - Test variable extraction from template
  - Test value parsing from semicolon string
  - Test path interpolation
  - Test cartesian product generation
  - Test validation (missing vars, invalid syntax)
  - Test edge cases (no vars, single var, many vars)

#### B. Integration Tests (UPDATE)
- Update `test/ut_local/` structure:
  ```
  test/ut_local_dynamic/
    ├── before/
    │   └── services/
    │       └── my-app/
    │           └── clusters/
    │               ├── alpha/
    │               │   ├── stg/
    │               │   └── prod/
    │               └── beta/
    │                   └── stg/
    └── after/
        └── (same structure with changes)
  ```
- Add new test in `Makefile`: `sit-test-dynamic`
- Test backward compatibility with old flags

#### C. Manual Testing
- Test with existing `test/local/` structure using old flags
- Test with new dynamic structure using new flags
- Test GitHub mode with both approaches

### 6. Documentation Updates

#### A. Update Files
- **README.md**: Update CLI examples with new flags
- **docs/DYNAMIC_KUST_PATH.md**: Mark as implemented, add examples
- **docs/TEMPLATE_VARIABLES.md**: Update field names (OverlayKeys instead of Environments)
- **sample/github-actions/*.yml**: Add examples with new flags

#### B. New Documentation
- **docs/MIGRATION_v0.5.md**: Migration guide from old to new flags
- **docs/EXAMPLES.md**: Various path configuration examples

### 7. Breaking Changes

**No backward compatibility** - old flags are removed immediately:
- Remove `--manifests-path`
- Remove `--service`
- Remove `--environments`

Users must migrate to new flags when upgrading to v0.5.

### 8. Implementation Phases

#### Phase 1: Core Implementation (Breaking Changes OK)
1. Create `pkg/pathbuilder` package with tests
2. Update `pkg/kustomize/builder.go` interface
3. Add new CLI flags to `main.go`
4. Update `runner/options.go`

#### Phase 2: Runner Integration
5. Update `runner/base.go` BuildManifests logic
6. Update `runner/local.go` (minimal changes)
7. Update `runner/github.go` checkout logic
8. Update models with FullBuildPath field

#### Phase 3: Testing
9. Write unit tests for pathbuilder
10. Create integration test fixtures
11. Test local mode end-to-end

#### Phase 4: Documentation
12. Update all documentation
13. Add migration guide
14. Update README examples

#### Phase 5: GitHub Mode (Future)
15. Update GitHub checkout strategy logic
16. Test GitHub mode
17. Update sample workflows

### 9. Potential Issues & Mitigations

**Issue 1: Path traversal security**
- **Risk**: User could inject `../../` in variables
- **Mitigation**: Validate that interpolated paths don't escape base directory

**Issue 2: GitHub checkout strategy selection**
- **Challenge**: Sparse checkout is faster for few combinations, but not all paths
- **Solution**: Auto-select strategy based on combination count:
  - `< 4 combinations`: Use sparse checkout with all specific paths (better performance)
  - `>= 4 combinations`: Use shallow checkout of entire repo (simpler, ensures all paths available)

**Issue 3: Long execution time**
- **Risk**: Many combinations could cause GitHub Actions timeout
- **Mitigation**: Users are responsible for their configuration. Document best practices.

### 10. Design Decisions

1. **Consistent Naming**: Use `OverlayKey` consistently across codebase instead of mixing Label/Environment

2. **Build Configuration in Reports**: Include `kustomizeBuildPath` and `kustomizeBuildValues` in report JSON for transparency

3. **Focus on Local Mode First**: Get local mode working completely before tackling GitHub mode complexities

4. **No Artificial Limits**: Users control their configuration, we don't enforce max combinations

5. **Direct Paths**: Local mode uses direct before/after paths, GitHub mode handles checkout separately

---

## Summary of Changes by File

| File | Type | Changes |
|------|------|---------|
| `pkg/pathbuilder/pathbuilder.go` | NEW | Core interpolation logic |
| `pkg/pathbuilder/pathbuilder_test.go` | NEW | Unit tests |
| `pkg/kustomize/builder.go` | MAJOR | Remove overlay logic, simplify to full path only |
| `cmd/gitops-kustomzchk/main.go` | MODERATE | Add new flags, migration |
| `internal/runner/options.go` | MODERATE | Add new fields, pathBuilder |
| `internal/runner/base.go` | MAJOR | Rewrite BuildManifests using pathBuilder |
| `internal/runner/github.go` | MODERATE | Update checkout logic |
| `internal/runner/local.go` | MINOR | Update path joining |
| `pkg/models/kustomize_result.go` | MODERATE | Rename to BuildOverlayManifestResult, add FullBuildPath |
| `pkg/models/reportdata.go` | MODERATE | Rename EnvironmentDiff to OverlayDiff, add build config fields |
| `docs/DYNAMIC_KUST_PATH.md` | MINOR | Mark as implemented |
| `docs/MIGRATION_v0.5.md` | NEW | Migration guide |
| `README.md` | MODERATE | Update examples |
| Test fixtures | NEW | Add dynamic path test cases |

---

## Implementation Priority

1. **Phase 1-3**: Focus on local mode - get it working end-to-end
2. **Phase 4**: Documentation for local mode usage
3. **Phase 5**: GitHub mode as future work
