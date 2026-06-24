package version

import "testing"

func TestDisplayVersionUsesBaseForDefaultBranch(t *testing.T) {
	for _, branch := range []string{"", "main", "refs/heads/main", "origin/main", "HEAD"} {
		if got := DisplayVersion("0.1", branch, "main"); got != "0.1" {
			t.Fatalf("DisplayVersion branch %q = %q, want 0.1", branch, got)
		}
	}
}

func TestDisplayVersionAddsSanitizedBranchSuffix(t *testing.T) {
	got := DisplayVersion("0.1", "fm/version-footer-f9", "main")
	if got != "0.1-fm-version-footer-f9" {
		t.Fatalf("DisplayVersion = %q, want 0.1-fm-version-footer-f9", got)
	}
}

func TestSanitizeBranch(t *testing.T) {
	tests := map[string]string{
		"dev":                               "dev",
		"refs/heads/Feature/Version Footer": "feature-version-footer",
		"origin/fm/version_footer.f9":       "fm-version-footer-f9",
		"---":                               "",
	}
	for branch, want := range tests {
		if got := SanitizeBranch(branch); got != want {
			t.Fatalf("SanitizeBranch(%q) = %q, want %q", branch, got, want)
		}
	}
}
