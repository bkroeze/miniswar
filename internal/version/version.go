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

// Base returns the ldflag-provided base version, then the embedded VERSION file,
// and finally 0.0 if neither source is available.
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

// Display returns the version string intended for UI display.
func Display() string {
	return DisplayVersion(Base(), branchName, defaultBranch)
}

// DisplayVersion returns the base version for default-branch builds and appends
// a sanitized branch suffix for non-default branch builds.
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

// SanitizeBranch normalizes a git branch name into a lower-case display suffix.
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
