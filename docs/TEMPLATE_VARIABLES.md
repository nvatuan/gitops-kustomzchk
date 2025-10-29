# Template Variables Reference

> **Source:** See `src/pkg/models/reportdata.go` for the complete data structure definitions.

## Template Files

Templates receive `ReportData` struct as root context:

- `comment.md.tmpl` - Main comment template
- `diff.md.tmpl` - Diff section template  
- `policy.md.tmpl` - Policy evaluation template

## Root Variables

```go
.Service          string              // Service name (e.g., "my-app")
.Timestamp        time.Time           // When check ran
.BaseCommit       string              // Base branch SHA (short)
.HeadCommit       string              // Head branch SHA (short)
.Environments     []string            // Environment list (e.g., ["stg", "prod"])
.ManifestChanges  map[string]EnvironmentDiff
.PolicyEvaluation PolicyEvaluation
```

## ManifestChanges (map[string]EnvironmentDiff)

Access via: `{{$diff := index .ManifestChanges "stg"}}`

```go
.LineCount          int       // Total changed lines
.AddedLineCount     int       // Added lines count
.DeletedLineCount   int       // Deleted lines count
.ContentType        string    // "text" or "ext_ghartifact"
.Content            string    // Diff text OR artifact URL
.ContentGHFilePath  *string   // GitHub artifact file path (if applicable)
```

## PolicyEvaluation

```go
.EnvironmentSummary  map[string]EnvironmentSummaryEnv
.PolicyMatrix        map[string]PolicyMatrix
```

### EnvironmentSummary (map[string]EnvironmentSummaryEnv)

Access via: `{{$sum := index .PolicyEvaluation.EnvironmentSummary "stg"}}`

```go
.PassingStatus {
  .PassBlockingCheck   bool
  .PassWarningCheck    bool
  .PassRecommendCheck  bool
}

.PolicyCounts {
  .TotalCount               int
  .TotalSuccess             int
  .TotalFailed              int
  .TotalOmitted             int
  .TotalOmittedFailed       int
  .TotalOmittedSuccess      int
  .BlockingSuccessCount     int
  .BlockingFailedCount      int
  .WarningSuccessCount      int
  .WarningFailedCount       int
  .RecommendSuccessCount    int
  .RecommendFailedCount     int
  .OverriddenSuccessCount   int
  .OverriddenFailedCount    int
  .NotInEffectSuccessCount  int
  .NotInEffectFailedCount   int
}
```

### PolicyMatrix (map[string]PolicyMatrix)

Access via: `{{$matrix := index .PolicyEvaluation.PolicyMatrix "prod"}}`

```go
.BlockingPolicies     []PolicyResult
.WarningPolicies      []PolicyResult
.RecommendPolicies    []PolicyResult
.OverriddenPolicies   []PolicyResult
.NotInEffectPolicies  []PolicyResult
```

### PolicyResult

```go
.PolicyId       string     // Policy identifier
.PolicyName     string     // Display name
.ExternalLink   string     // Optional documentation URL
.IsPassing      bool       // true if passed
.FailMessages   []string   // Failure details
```

## Template Functions

```go
{{if gt .LineCount 0}}                    // Greater than
{{if eq .ContentType "text"}}             // Equal
{{range .Environments}}                   // Iterate
{{$diff := index .ManifestChanges $env}}  // Map access
{{.Timestamp.Format "2006-01-02"}}        // Time format
```

## Usage Examples

### Iterate environments and show diffs

```go
{{range $env := .Environments}}
  {{$diff := index $.ManifestChanges $env}}
  ### {{$env}}
  Lines: {{$diff.LineCount}} (+{{$diff.AddedLineCount}} -{{$diff.DeletedLineCount}})
  {{if eq $diff.ContentType "text"}}
    ```diff
    {{$diff.Content}}
    ```
  {{else}}
    [View diff artifact]({{$diff.Content}})
  {{end}}
{{end}}
```

### Policy summary table

```go
| Environment | Success | Failed | Blocking❌ | Warning⚠️ | Recommend💡 |
|-------------|---------|--------|-----------|----------|------------|
{{range $env, $sum := .PolicyEvaluation.EnvironmentSummary -}}
| {{$env}} | {{$sum.PolicyCounts.TotalSuccess}} | {{$sum.PolicyCounts.TotalFailed}} | {{$sum.PolicyCounts.BlockingFailedCount}} | {{$sum.PolicyCounts.WarningFailedCount}} | {{$sum.PolicyCounts.RecommendFailedCount}} |
{{end}}
```

### Policy matrix with external links

```go
{{$matrix := index .PolicyEvaluation.PolicyMatrix "stg"}}
{{range $policy := $matrix.BlockingPolicies}}
  {{if $policy.ExternalLink}}
    - [{{$policy.PolicyName}}]({{$policy.ExternalLink}})
  {{else}}
    - {{$policy.PolicyName}}
  {{end}}
  {{if not $policy.IsPassing}}
    {{range $policy.FailMessages}}
      - ❌ {{.}}
    {{end}}
  {{end}}
{{end}}
```

### Cross-environment policy comparison

```go
| Policy | stg | prod |
|--------|-----|------|
{{range $policy := (index .PolicyEvaluation.PolicyMatrix "stg").BlockingPolicies -}}
  {{$prodMatrix := index $.PolicyEvaluation.PolicyMatrix "prod" -}}
  {{$prodPolicy := "" -}}
  {{range $p := $prodMatrix.BlockingPolicies -}}
    {{if eq $p.PolicyId $policy.PolicyId}}{{$prodPolicy = $p}}{{end -}}
  {{end -}}
| {{$policy.PolicyName}} | {{if $policy.IsPassing}}✅{{else}}❌{{end}} | {{if $prodPolicy.IsPassing}}✅{{else}}❌{{end}} |
{{end}}
```

## Example Output (JSON)

```json
{
  "service": "my-app",
  "timestamp": "2025-10-29T15:24:38.440679+09:00",
  "baseCommit": "abc1234",
  "headCommit": "def5678",
  "environments": ["stg", "prod"],
  "manifestChanges": {
    "stg": {
      "lineCount": 16,
      "addedLineCount": 12,
      "deletedLineCount": 4,
      "contentType": "text",
      "content": "--- before\t2025-10-29 15:24:38\n+++ after\t2025-10-29 15:24:38\n..."
    },
    "prod": {
      "lineCount": 36,
      "addedLineCount": 32,
      "deletedLineCount": 4,
      "contentType": "text",
      "content": "--- before\t2025-10-29 15:24:38\n+++ after\t2025-10-29 15:24:38\n..."
    }
  },
  "policyEvaluation": {
    "environmentSummary": {
      "stg": {
        "passingStatus": {
          "passBlockingCheck": true,
          "passWarningCheck": false,
          "passRecommendCheck": false
        },
        "policyCounts": {
          "totalCount": 5,
          "totalSuccess": 3,
          "totalFailed": 2,
          "blockingFailedCount": 0,
          "warningFailedCount": 1,
          "recommendFailedCount": 1
        }
      },
      "prod": {
        "passingStatus": {
          "passBlockingCheck": false,
          "passWarningCheck": false,
          "passRecommendCheck": false
        },
        "policyCounts": {
          "totalCount": 5,
          "totalSuccess": 2,
          "totalFailed": 3,
          "blockingFailedCount": 1,
          "warningFailedCount": 1,
          "recommendFailedCount": 1
        }
      }
    },
    "policyMatrix": {
      "stg": {
        "blockingPolicies": [
          {
            "policyId": "service-persistent-volume-forbidden",
            "policyName": "Service Persistent Volume Forbidden",
            "isPassing": true,
            "failMessages": []
          },
          {
            "policyId": "service-taggings",
            "policyName": "Service Taggings",
            "isPassing": true,
            "failMessages": []
          }
        ],
        "warningPolicies": [
          {
            "policyId": "service-high-availability",
            "policyName": "Service High Availability",
            "externalLink": "https://example.com/docs/high-availability",
            "isPassing": false,
            "failMessages": [
              "Deployment 'stg-my-app' must have PodAntiAffinity or PodTopologySpread for high availability"
            ]
          }
        ],
        "recommendPolicies": [
          {
            "policyId": "service-no-cpu-limit",
            "policyName": "Service No CPU Limit",
            "isPassing": false,
            "failMessages": [
              "Deployment 'stg-my-app' container 'my-app' should not have a cpu limit, found: 800m"
            ]
          }
        ],
        "overriddenPolicies": [],
        "notInEffectPolicies": []
      },
      "prod": {
        "blockingPolicies": [
          {
            "policyId": "service-persistent-volume-forbidden",
            "policyName": "Service Persistent Volume Forbidden",
            "isPassing": true,
            "failMessages": []
          },
          {
            "policyId": "service-taggings",
            "policyName": "Service Taggings",
            "isPassing": false,
            "failMessages": [
              "CronJob prod-hello-world-cronjob does not have the required label 'github.com/nvatuan/domains'",
              "Deployment prod-my-app does not have the required label 'github.com/nvatuan/domains'"
            ]
          }
        ],
        "warningPolicies": [
          {
            "policyId": "service-high-availability",
            "policyName": "Service High Availability",
            "externalLink": "https://example.com/docs/high-availability",
            "isPassing": false,
            "failMessages": [
              "Deployment 'prod-my-app' must have PodAntiAffinity or PodTopologySpread for high availability"
            ]
          }
        ],
        "recommendPolicies": [
          {
            "policyId": "service-no-cpu-limit",
            "policyName": "Service No CPU Limit",
            "isPassing": true,
            "failMessages": []
          }
        ],
        "overriddenPolicies": [],
        "notInEffectPolicies": []
      }
    }
  }
}
```

## Testing

```bash
# Test templates locally
make run-local

# View generated report
cat test/output/report.md
cat test/output/report.json
```
