// Package cleaner provides leak detection patterns for scanning OPCT archives.
//
// Patterns are sourced from the leaktk/patterns project:
//
//	https://github.com/leaktk/patterns
//
// To update patterns, fetch the latest merged TOML from:
//
//	https://github.com/leaktk/patterns/blob/main/patterns/gitleaks/8.27.0/98-general.toml
//
// LeakTK documentation:
//
//	https://github.com/leaktk/leaktk
//	https://github.com/leaktk/leaktk/blob/main/docs/scan.md
package cleaner

import "regexp"

// LeakPattern defines a single leak detection rule.
type LeakPattern struct {
	ID          string
	Description string
	Regex       *regexp.Regexp
	Keywords    []string
}

// LeakFinding represents a detected potential leak in the archive.
type LeakFinding struct {
	File    string
	Pattern string
	Line    int
}

// leakPatterns is the curated set of high-priority leak detection patterns
// for OpenShift cluster archives. Each pattern is sourced from leaktk/patterns
// (gitleaks v8.27.0 format).
//
//nolint:lll
var leakPatterns = []LeakPattern{
	// Source: https://github.com/leaktk/patterns — Pattern ID: sOZiHxUBVFc
	{
		ID:          "sOZiHxUBVFc",
		Description: "OpenShift User Token",
		Regex:       regexp.MustCompile(`\b(sha256~[\w\-]{43})(?:[^\w\-]|\z)`),
		Keywords:    []string{"sha256~"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: vAAom0bPHy8
	{
		ID:          "vAAom0bPHy8",
		Description: "Kubernetes Service Account JWT",
		Regex:       regexp.MustCompile(`[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+(?:InN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudD|JzdWIiOiJzeXN0ZW06c2VydmljZWFjY291bnQ6|ic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50O)[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+`),
		Keywords:    []string{"inn1yii6inn5c3rlbtpzzxj2awnlywnjb3vudd", "jzdwiioijzexn0zw06c2vydmljzwfjy291bnq6", "ic3viijoic3lzdgvtonnlcnzpy2vhy2nvdw50o"},
	},
	// Broader JWT pattern for OIDC/audience tokens (not just service account)
	// Matches any JWT with RS256 algorithm header (base64 of {"alg":"RS256")
	{
		ID:          "opct-jwt-broad",
		Description: "JWT Token (RS256)",
		Regex:       regexp.MustCompile(`eyJhbGciOiJSUzI1NiIs[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]{50,}\.[a-zA-Z0-9\-_]+`),
		Keywords:    []string{"eyjhbgcioijsuzi1niis"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: gpfGmO3HH64
	{
		ID:          "gpfGmO3HH64",
		Description: "Container Registry Authentication",
		Regex:       regexp.MustCompile(`\\*"auths\\*"\s*:\s*\{\s*(?:\\*"(?:[a-z0-9\-]{1,63}\.)+(?:[a-z0-9\-]{1,63})\\*"\s*:\s*\{\s*\\*"auth\\*"\s*:\s*\\*"[\w\/+\-]{32,}={0,2}\\*"[\s\S]*?\},?\s*)+\}`),
		Keywords:    []string{`"auths`},
	},
	// Broader registry auth pattern for unescaped JSON (e.g., machineconfigs.json)
	{
		ID:          "opct-registry-auth-unescaped",
		Description: "Container Registry Authentication (unescaped)",
		Regex:       regexp.MustCompile(`"auths"\s*:\s*\{\s*"(?:[a-z0-9\-]{1,63}[\.:])+(?:[a-z0-9\-]{1,63})(?::\d+)?"\s*:\s*\{\s*"auth"\s*:\s*"[\w\/+\-]{32,}={0,2}"`),
		Keywords:    []string{`"auths"`},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: LAJoYTdoQH4
	{
		ID:          "LAJoYTdoQH4",
		Description: "AWS IAM Unique Identifier",
		Regex:       regexp.MustCompile(`(?:^|[^!$-&\(-9<>-~])((?:A3T[A-Z0-9]|ACCA|ABIA|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z2-7]{16})\b`),
		Keywords:    []string{"a3t", "abia", "acca", "agpa", "aida", "aipa", "akia", "anpa", "anva", "aroa", "asia"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: 9j_rmwDeioM
	{
		ID:          "9j_rmwDeioM",
		Description: "AWS Secret Access Key",
		Regex:       regexp.MustCompile(`(?i)aws[\w\-]{0,32}['\"]?\s*?[:=\(]\s*?['\"]?([a-z0-9\/+]{40})(?:[^a-z0-9\/+]|\z)`),
		Keywords:    []string{"aws"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: zl044yuux24
	{
		ID:          "zl044yuux24",
		Description: "Azure AD Client Secret",
		Regex:       regexp.MustCompile(`(?:[^a-zA-Z0-9_~.\-]|\A)([a-zA-Z0-9_~.\-]{3}\dQ~[a-zA-Z0-9_~.\-]{31,34})(?:[^a-zA-Z0-9_~.\-]|\z)`),
		Keywords:    []string{"q~"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: HysINeDft8k
	{
		ID:          "HysINeDft8k",
		Description: "GCP API Key",
		Regex:       regexp.MustCompile(`\b(AIza[\w\\\-]{35})(?:[^\w\\\-]|$)`),
		Keywords:    []string{"aiza"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: ePK9whPQPpY
	{
		ID:          "ePK9whPQPpY",
		Description: "Private Key (PEM)",
		Regex:       regexp.MustCompile(`(?i)-----BEGIN[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----[\s\S]*?(?:[a-z0-9\/+]{64}[\s\S]*?){2}-----END[ A-Z0-9_-]{0,100}PRIVATE KEY(?: BLOCK)?-----`),
		Keywords:    []string{"-----begin"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: _-9w6-yrc-4
	{
		ID:          "_-9w6-yrc-4",
		Description: "Generic Secret (key=value quoted)",
		Regex:       regexp.MustCompile(`(?i)[\w\-]*(?:(?:password|secret|token)[_\-]?(?:access[_\-]?)?(?:key)?|api[_\-]?key)[\"']?\s*?\]?\s*?[:=]\s*?[\"']([^\"\s]+?)[\"']`),
		Keywords:    []string{"key", "password", "secret", "token"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: hG-qMjbXGro
	{
		ID:          "hG-qMjbXGro",
		Description: "Generic Secret (key=value unquoted)",
		Regex:       regexp.MustCompile(`(?i)(?:^|\n)[#\/\s]*[\w\-]*(?:(?:password|secret|token)_?(?:access_?)?(?:key)?|api_?key)=([^\s\"',]{6,})`),
		Keywords:    []string{"key", "password", "secret", "token"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: gODCNuGzuKQ
	{
		ID:          "gODCNuGzuKQ",
		Description: "GitHub Personal Access Token",
		Regex:       regexp.MustCompile(`\bghp_[0-9A-Za-z]{36}\b`),
		Keywords:    []string{"ghp_"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: kX_PwM0MFvE
	{
		ID:          "kX_PwM0MFvE",
		Description: "GitHub Fine-Grained PAT",
		Regex:       regexp.MustCompile(`\bgithub_pat_\w{82}\b`),
		Keywords:    []string{"github_pat_"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: rnWF160pWNg
	{
		ID:          "rnWF160pWNg",
		Description: "Generic Secret (YAML)",
		Regex:       regexp.MustCompile(`(?i)[\w\-]*(?:password|secret|token)[_-]?(?:access[_-]?)?(?:key)?:[\t ]+?([^\"\'\s]+?)[\t ]*(?:\\n|\n|#|$)`),
		Keywords:    []string{"password", "secret", "token"},
	},
	// Source: https://github.com/leaktk/patterns — Pattern ID: QqS4RvI6Zmg
	{
		ID:          "QqS4RvI6Zmg",
		Description: "Authorization Header",
		Regex:       regexp.MustCompile(`(?i)(?:\A|[^\w\-])Authorization:[\t ]*(?:\w+[\t ]+)?([^\s\"\'<]{18,})`),
		Keywords:    []string{"authorization"},
	},
}
