## UT_GITHUB

Integration testing for GitHub Mode - validates the full GitHub runner workflow end-to-end.

### Structure

```
test/ut_github/
├── before/              # Base K8s manifests (represents target branch)
│   └── services/
│       └── my-app/
│           ├── base/
│           └── environments/
├── after/               # Changed manifests (represents PR changes)
│   └── services/
│       └── my-app/
│           └── environments/
├── policies/            # OPA/Rego policies for validation
│   ├── compliance-config.yaml
│   ├── ha.rego
│   ├── taggings.rego
│   └── no_cpu_limit.rego
├── templates/           # Report templates
│   ├── comment.md.tmpl
│   ├── diff.md.tmpl
│   └── policy.md.tmpl
└── jury_output/         # Expected output for validation
    ├── report.md
    └── report.json
```

### How It Works

The GitHub integration test validates the entire workflow in a real GitHub environment:

1. **Test Orchestrator** (`test-github-integration.yml`)
   - Creates two temporary branches:
     - Base branch: contains `before/` manifests
     - Test branch: contains `after/` manifests (changes)
   - Creates a test PR from test branch to base branch
   - Dispatches the **GitOps Runner** workflow via `workflow_dispatch`
   - Waits for runner to complete
   - Fetches PR comments via GitHub API
   - Validates comment content against expected structure
   - Compares results with `jury_output/report.md`
   - Cleans up test PR and branches

2. **GitOps Runner** (`test-github-runner.yml`)
   - Triggered by workflow_dispatch from orchestrator
   - Receives PR number and git refs as inputs
   - Builds and runs `gitops-kustomzchk` in GitHub mode
   - Posts analysis comment to the test PR
   - Signals completion

### Test Scenarios

The test data includes:

**Prod Environment:**
- Image change: `nginx:1.21` → `nginx:latest`
- CPU limit removed
- New CronJob added
- Ingress host changed

**Stg Environment:**
- Labels added: `github.com/nvatuan/domains`
- Log level changed: `debug` → `warn`
- Resource limits increased
- KEDA minReplicaCount: `1` → `4`

**Expected Policy Results:**
- `prod`: 1 blocking failure (missing tags), 1 warning (no HA)
- `stg`: 1 warning (no HA), 1 recommendation (has CPU limit)

### Running Tests

Tests run automatically on:
- **Push** to `main`, `develop`, or `feature/**` branches
- **Pull requests** that change:
  - Source code (`src/**`)
  - Test data (`test/ut_github/**`)
  - Test workflows (`.github/workflows/test-github-*.yml`)
- **Manual dispatch** via GitHub Actions UI

### Manual Execution

To manually trigger the test:

```bash
gh workflow run test-github-integration.yml
```

Or via GitHub UI:
1. Go to Actions → "Test GitHub Integration"
2. Click "Run workflow"
3. Select branch
4. Click "Run workflow"

### Validation Logic

The test validates:

1. **Comment Structure**: Checks for required sections:
   - GitOps Policy Check header
   - Summary table
   - Service/Environment info
   - Manifest Changes
   - Policy Evaluation

2. **Policy Results**: Compares violation counts with jury output

3. **Workflow Completion**: Ensures runner workflow completes successfully

### Updating Test Data

To update test scenarios:

1. **Modify manifests:**
   - `before/` - base state
   - `after/` - changed state

2. **Update policies:**
   - Edit `.rego` files in `policies/`
   - Update `compliance-config.yaml`

3. **Regenerate jury output:**
   ```bash
   # Run locally to generate expected output
   make sit-test-local
   cp test/ut_local/output/report.md test/ut_github/jury_output/
   cp test/ut_local/output/report.json test/ut_github/jury_output/
   ```

### Troubleshooting

**Test fails with "No comments found":**
- Check runner workflow logs
- Verify PR was created successfully
- Ensure `GITHUB_TOKEN` has write permissions

**Comment validation fails:**
- Compare actual comment with jury output
- Check if test data changed unexpectedly
- Verify template files are correct

**Workflow timeout:**
- Increase `MAX_WAIT` in orchestrator workflow
- Check for hung runner workflow
- Review GitHub Actions quotas

### Cleanup

Test artifacts are automatically cleaned up:
- Test PR is closed
- Test branches are deleted

If cleanup fails, manually delete branches matching pattern:
```bash
git push origin --delete test/gh-integration-base-*
git push origin --delete test/gh-integration-test-*
```