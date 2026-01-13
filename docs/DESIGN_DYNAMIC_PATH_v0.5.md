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

**Replace:**
- ~~`--manifests-path`~~
- ~~`--service`~~ (keep for backward compat but deprecate)
- ~~`--environments`~~ (keep for backward compat but deprecate)

**Add:**
- `--kustomize-build-path` (string): Template path with `$VARIABLE` placeholders
  - Example: `/home/me/k8s-manifests/services/$SERVICE/clusters/$CLUSTER/envs/$ENV`
- `--kustomize-build-values` (string): Semicolon-separated key-value assignments
  - Format: `KEY=value1,value2;KEY2=value3`
  - Example: `SERVICE=my-app;CLUSTER=alpha,beta;ENV=stg,prod`

**Backward Compatibility Mode:**
- If old flags used, internally convert to new format:
  ```
  --kustomize-build-path = "<manifests-path>/$SERVICE/environments/$ENV"
  --kustomize-build-values = "SERVICE=<service>;ENV=<env1>,<env2>"
  ```

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

type PathCombination struct {
    Path   string              // Full interpolated path
    Values map[string]string   // Variable values used
    Label  string              // Human-readable label (e.g., "my-app/alpha/stg")
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
    // Deprecated (keep for backward compat)
    Service       string   // DEPRECATED
    Environments  []string // DEPRECATED
    ManifestsPath string   // DEPRECATED

    // New fields
    KustomizeBuildPath   string // Template with $VARs
    KustomizeBuildValues string // "KEY=v1,v2;KEY2=v3"
    
    // Computed internally (not CLI flags)
    pathBuilder *pathbuilder.PathBuilder // Parsed path builder
}
```

#### D. Update `cmd/gitops-kustomzchk/main.go`
```go
// Add new flags
cmd.Flags().StringVar(&opts.KustomizeBuildPath, "kustomize-build-path", "", 
    "Path template with $VARIABLES (e.g., '/path/$SERVICE/clusters/$CLUSTER/$ENV')")
cmd.Flags().StringVar(&opts.KustomizeBuildValues, "kustomize-build-values", "",
    "Variable values: 'KEY=v1,v2;KEY2=v3' (e.g., 'SERVICE=my-app;ENV=stg,prod')")

// Deprecation warnings for old flags
// Migration logic to convert old -> new format
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
    
    results := make(map[string]models.BuildEnvManifestResult)
    
    for _, combo := range pathCombos {
        // Build for "before" and "after" using combo.Path
        beforeFullPath := filepath.Join(beforeRoot, combo.Path)
        afterFullPath := filepath.Join(afterRoot, combo.Path)
        
        beforeManifest, err := r.Builder.Build(ctx, beforeFullPath)
        afterManifest, err := r.Builder.Build(ctx, afterFullPath)
        
        // Use combo.Label as the "environment" key
        results[combo.Label] = models.BuildEnvManifestResult{
            Environment:    combo.Label,
            BeforeManifest: beforeManifest,
            AfterManifest:  afterManifest,
        }
    }
    
    return &models.BuildManifestResult{EnvManifestBuild: results}, nil
}
```

#### F. Update `internal/runner/github.go`
```go
func (r *RunnerGitHub) Process() error {
    // CHANGED: Checkout strategy needs updating
    // Instead of checking out specific service path,
    // extract base path from template (everything before first $VAR)
    
    basePath := r.Options.pathBuilder.GetBasePathForCheckout()
    // Checkout basePath instead of manifests+service
    
    // Rest remains similar but uses pathBuilder.GenerateAllPaths()
}
```

### 3. Model Changes

#### A. `pkg/models/kustomize_result.go`
```go
type BuildEnvManifestResult struct {
    Environment    string // NOW: Full label like "my-app/clusters/alpha/stg"
    FullBuildPath  string // NEW: Store the actual path used for building
    BeforeManifest []byte
    AfterManifest  []byte
    Skipped        bool
    SkipReason     string
}
```

#### B. `pkg/models/reportdata.go`
```go
// No struct changes needed
// Templates already use .Environment as key
// But template rendering will show richer labels
```

### 4. Template Changes

Update default templates to use the new label format:

**`src/templates/diff.md.tmpl`**
```markdown
### [`{{$env}}`]: {{if gt $diff.LineCount 0}}...
```
This already works! `$env` will just be the richer label.

**`src/templates/comment.md.tmpl`** & **`policy.md.tmpl`**
- No changes needed, they iterate over environments dynamically

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
- **docs/TEMPLATE_VARIABLES.md**: Update `.Environment` description
- **sample/github-actions/*.yml**: Add examples with new flags

#### B. New Documentation
- **docs/MIGRATION_v0.5.md**: Migration guide from old to new flags
- **docs/EXAMPLES.md**: Various path configuration examples

### 7. Backward Compatibility

#### Strategy
1. **Keep old flags** for 2 versions (v0.5, v0.6)
2. **Add deprecation warnings** in logs when old flags used
3. **Internal migration**: Convert old flags to new format automatically
4. **Remove in v0.7**: Clean up old code

#### Implementation
```go
func (opts *Options) MigrateOldFlags() {
    if opts.KustomizeBuildPath == "" && opts.Service != "" {
        // Old flags detected
        logger.Warn("DEPRECATED: --service, --environments, --manifests-path are deprecated. Use --kustomize-build-path and --kustomize-build-values instead.")
        
        opts.KustomizeBuildPath = filepath.Join(opts.ManifestsPath, "$SERVICE", "environments", "$ENV")
        
        envList := strings.Join(opts.Environments, ",")
        opts.KustomizeBuildValues = fmt.Sprintf("SERVICE=%s;ENV=%s", opts.Service, envList)
    }
}
```

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

#### Phase 3: Backward Compatibility
9. Implement flag migration logic
10. Add deprecation warnings
11. Test both old and new flag approaches

#### Phase 4: Testing & Documentation
12. Write unit tests for pathbuilder
13. Create integration test fixtures
14. Update all documentation
15. Add migration guide
16. Update sample workflows

#### Phase 5: Polish
17. Update templates (if needed)
18. Performance testing
19. Security review (path traversal attacks?)
20. Final integration testing

### 9. Potential Issues & Mitigations

**Issue 1: Path traversal security**
- **Risk**: User could inject `../../` in variables
- **Mitigation**: Validate that interpolated paths don't escape base directory

**Issue 2: Combinatorial explosion**
- **Risk**: `$A=1,2,3;$B=1,2,3;$C=1,2,3` → 27 builds
- **Mitigation**: Add flag `--max-build-combinations` (default: 50)

**Issue 3: GitHub checkout with dynamic paths**
- **Risk**: Sparse checkout needs to know what to checkout upfront
- **Mitigation**: Extract common base path from template (everything before first variable)

**Issue 4: Report readability**
- **Risk**: Labels like `my-app/clusters/alpha/overlays/stg/final` too long
- **Mitigation**: Add optional `--environment-labels` to override display names

**Issue 5: Existing PR comments**
- **Risk**: Changing environment labels breaks comment updates
- **Mitigation**: Include label hash in comment signature, or use service name

### 10. Open Questions for Review

1. **Should we support custom label format?**
   - Proposal: Add `--environment-label-format` flag
   - Example: `--environment-label-format="$CLUSTER-$ENV"` → "alpha-stg"
   - Default: Use relative path from manifests root

2. **What about `Service` field in reports?**
   - Keep it as required? Or make it optional and derive from path?
   - Backward compat: Extract SERVICE from variables if exists

3. **GitHub Actions workflow changes?**
   - Do we need to detect changed files differently?
   - Current: Assumes `services/<service>/environments/<env>` structure

4. **Should we validate max path combinations?**
   - Hard limit? Soft warning?
   - What's a reasonable default?

5. **How to handle local vs GitHub mode differences?**
   - Local: Direct paths (before/after roots)
   - GitHub: Checkout + path joining
   - Should they use same interface?

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
| `pkg/models/kustomize_result.go` | MINOR | Add FullBuildPath field |
| `docs/DYNAMIC_KUST_PATH.md` | MINOR | Mark as implemented |
| `docs/MIGRATION_v0.5.md` | NEW | Migration guide |
| `README.md` | MODERATE | Update examples |
| Test fixtures | NEW | Add dynamic path test cases |

---

## Questions for You

1. **Backward compatibility duration**: Keep old flags for how many versions?
2. **Label format**: Should we support custom label formatting or just use path-based labels?
3. **GitHub Actions**: Do you have specific workflows that need updating?
4. **Service field**: Keep it required or derive from variables?
5. **Security**: Should we validate/sanitize path variables?
6. **Priority**: Any specific use case we should test first?
