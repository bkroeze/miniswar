package version

import (
	_ "embed"
	"strings"
	"unicode"
)

//go:embed VERSION
var embeddedBaseVersion string

var (
	baseVersion   string
	branchName    string
	defaultBranch = "main"
)

func Base() string {
	base := strings.TrimSpace(baseVersion)
	if base == "" {
		base = strings.TrimSpace(embeddedBaseVersion)
	}
	if base == "" {
		return "0.0"
	}
	return base
}

func Display() string {
	return DisplayVersion(Base(), branchName, defaultBranch)
}

func DisplayVersion(base, branch, defaultBranch string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "0.0"
	}
	branch = strings.TrimSpace(branch)
	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if branch == "" || branch == "HEAD" {
		return base
	}
	suffix := SanitizeBranch(branch)
	if suffix == "" || suffix == SanitizeBranch(defaultBranch) {
		return base
	}
	return base + "-" + suffix
}

func SanitizeBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	branch = strings.TrimPrefix(branch, "origin/")

	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(branch) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '.' || r == '_' || r == '-':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
