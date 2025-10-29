# gitops-kustomz

## Usecase

The tool is designed to run in GitHub Actions CI on Pull Requests. It analyzes k8s manifest changes, run certain checks on it like enforces OPA policies.

As of now, the intended usecase for this tool is to:

1. PR open
2. [GitHub Action Script] Get service and environment that is being changed, extract the value
3. [GitHub Action Script] For each combination of <env>-<service>, trigger a job with those params to the tool
4. [gitops-kustomz] For better GitHub comment management, it will start by writing one comment saying this comment is a placeholder and will be updated in the next few steps.
5. [gitops-kustomz] Build step; Run kustomize build on <env>-<service> on Base branch, and Head Branch 
6. [gitops-kustomz] Diff step; Execute a diff on kustomize build of BASE and HEAD then update comment in 4th step in the DIFF section
7. [gitops-kustomz] Policy Eval step; Gather OPA policies defined in a location, evaluate them, then update the comment in 4th step with the Policy Evaluation result report

## Detailed Design

- Since the tool is designed to run on PR, it will need a PR URL or repository with PR number in order to run.
- It can try use GH_TOKEN variable, but in case not possible (403, 401,..) tool exists with error message indicate GitHub credentials is not valid
- Tool is designed with the kustomize structure (base and overlays, or base and environments) in mind, for example, this structure works:

```
- services/
|- my-app/
|  |  - base/
|  |  |  - kustomization.yaml  
|  |  |  - deployment.yaml
|  |  |  - ingress.yaml
|  |  - environments/
|  |  |  |  - test/
|  |  |  |  |  - kustomization.yaml
|  |  |  |  |  - deployment.yaml
|  |  |  |  - prod/
|  |  |  |  |  - kustomization.yaml
|  |  |  |  |  - deployment.yaml

```

### Highly Configurable

- GitHub comment template (4th step) is just a template in `ghcomment.md.tmpl` format file, binary will read this file to get the template.
- GitHub comment message of 5th and 6th step should be configurable where it can.
- For Policies step, it use a `compliance-config.yaml` (check sample/policies/compliance-config.yaml) to configure levels of enforcement and which OPA file to use.
  - The tool then gather, group OPA policies together, and evaluate them, then enforce them based on configuration. (eg. if a policy require failing CI when not passing, the tool should respect that)
  - Of course, a report of policy evaluation should be created and updated to 4th step, and this template should be configurable
- Some policy can be set to be ignored (not evaluated) if on the PR, there are comments that indicate to override them.
  - This is configurable in the compliance config

### Optimized for CI

- This tool should execute quickly
- Cache when it can, for example, it can try catch kustomize build based on SHA commit code of the base branch.
- Only runs when it needs: for example, if people commenting on the PR but it doesn't contain override messages then the tool shouldn't retry building kustomize manifests and re-evaluate policies.

### Production Ready

- Must be well-tested with good structure of unit testing

### Easy Maintenance

- This tool should provide testing for when writing OPA policies. It can use `conftest` below the system.
- It must support testing in Local where CI commenting is not possible by provide some short of interface instead of the PR, for example, a structure of folder with before, after with kustomize manifest inside, after triggers, the tool output some markdown files and developers can think that those output are github comments.
- It must apply programming patterns that is allow high expendability


