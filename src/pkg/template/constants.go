package template

// DefaultCommentTemplate is the embedded default template for PR comments
// This template supports MultiEnvCommentData structure
const (
	ToolCommentSignature    = `<!-- gitops-kustomzchk: {{.Service}} - auto-generated comment, please do not remove -->`
	FileNameCommentTemplate = "comment.md.tmpl"
	FileNameDiffTemplate    = "diff.md.tmpl"
	FileNamePolicyTemplate  = "policy.md.tmpl"
)
