This is test fixture for local testing for version before v0.4 inclusive, before introduction of DYNAMIC build path

## Version

```bash
❯ opa version
Version: 1.3.0
Build Commit: 
Build Timestamp: 
Build Hostname: 
Go Version: go1.24.1
Platform: darwin/arm64
Rego Version: v1
WebAssembly: unavailable
❯ conftest -v
Conftest: 0.59.0
OPA: 1.3.0
```

## Usage

```bash
❯ conftest test --all-namespaces --combine --policy policies/ha.rego prod.yaml
FAIL - Combined - main - Deployment 'prod-my-app' must have PodAntiAffinity with topologyKey 'kubernetes.io/hostname' for high availability
FAIL - Combined - main - Deployment 'prod-my-app' must have at least 2 replicas for high availability, found: 1

❯ conftest verify --policy policies/ha.rego --policy policies/ha_test.rego

8 tests, 8 passed, 0 warnings, 0 failures, 0 exceptions, 0 skipped
```

 * use `--combine`, inputs shape also change so be careful

* Use command
```bash
conftest test --all-namespaces --combine --policy policies/ha.rego kbuild-prod.yaml -o json
```

* Output
```json
[
  {
    "filename": "Combined",
    "namespace": "main",
    "successes": 2,
    "failures": [
      {
        "msg": "Deployment 'prod-my-app' must have at least 2 replicas for high availability, found: 1",
        "metadata": {
          "query": "data.main.deny"
        }
      }
    ]
  }
]
```