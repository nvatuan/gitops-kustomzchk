# Git Checkout Strategy

## Overview

The `--git-checkout-strategy` flag allows you to control how the tool clones repositories when running in GitHub mode. This is particularly important for monorepos with cross-service dependencies.

## Strategies

### `sparse` (Default)

**Use when:** Your service is self-contained within its own directory with no cross-service kustomize dependencies.

**Behavior:**
- Clones only the specified service directory using git sparse-checkout
- Faster clone times
- Minimal disk usage
- Uses `--filter=blob:none` for treeless clones

**Example:**
```bash
gitops-kustomzchk \
  --run-mode github \
  --git-checkout-strategy sparse \
  --service my-app \
  # ... other flags
```

### `shallow`

**Use when:** Your service has kustomize references to other services (e.g., `../../other-service/base/components`).

**Behavior:**
- Clones all files in the repository with depth 1
- Ensures all cross-service dependencies are available
- Slower clone, more disk usage
- Necessary for services with external kustomize references

**Example:**
```bash
gitops-kustomzchk \
  --run-mode github \
  --git-checkout-strategy shallow \
  --service payable \
  # ... other flags
```

## GitHub Actions Integration

In your GitHub Actions workflow, you can configure this per service:

```yaml
jobs:
  kustomzcheck:
    strategy:
      matrix:
        include:
          # Most services use sparse (default, faster)
          - service: my-app
            checkout-strategy: sparse
          
          # Services with cross-service deps use shallow
          - service: my-app2
            checkout-strategy: shallow
          - service: my-app3
            checkout-strategy: shallow
    
    steps:
      - name: Run policy check
        run: |
          gitops-kustomzchk \
            --run-mode github \
            --git-checkout-strategy ${{ matrix.checkout-strategy }} \
            --service ${{ matrix.service }} \
            # ... other flags
```

## Implementation Details

### Sparse Checkout
```bash
git clone --filter=blob:none --depth 1 --no-checkout --single-branch -b <branch> <url> <dir>
git sparse-checkout set --no-cone <path>
git checkout <branch>
```

### Shallow Checkout
```bash
git clone --depth 1 --single-branch -b <branch> <url> <dir>
```

## FAQ

**Q: Can I use sparse with cross-service dependencies?**
A: No. Sparse checkout only fetches the specified path. Any kustomize references outside that path will fail.

**Q: Why not always use shallow?**
A: Shallow clones are slower and use more disk space. For most self-contained services, sparse is sufficient and faster.

**Q: Does this affect local mode?**
A: No. The `--git-checkout-strategy` flag only applies to GitHub mode. Local mode uses directories that already exist.

**Q: What if I'm not sure which strategy to use?**
A: Start with `sparse` (default). If you encounter "no such file or directory" errors during kustomize build, switch to `shallow`.

```
Error: accumulating resources: ... 
lstat /path/to/services/other-service: no such file or directory
```

This indicates your service has kustomize references to `other-service` which wasn't included in the sparse checkout. Switch to `--git-checkout-strategy shallow`.

