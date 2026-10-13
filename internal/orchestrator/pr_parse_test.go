package orchestrator

import "testing"

func TestParseAegisBranch(t *testing.T) {
	tests := map[string]int64{
		"aegis/fix-42":  42,
		"aegis/feat-7":  7,
		"main":          0,
		"aegis/fix-abc": 0,
	}
	for branch, want := range tests {
		if got := parseAegisBranch(branch); got != want {
			t.Fatalf("parseAegisBranch(%q) = %d, want %d", branch, got, want)
		}
	}
}

func TestParseAegisBranchFromRef(t *testing.T) {
	if got := ParseAegisBranchFromRef("refs/heads/aegis/fix-7"); got != 7 {
		t.Fatalf("ParseAegisBranchFromRef() = %d", got)
	}
}

func TestParseFixesIssueReference(t *testing.T) {
	body := "This PR Fixes #123 for the signup bug"
	if got := parseFixesIssueReference(body); got != 123 {
		t.Fatalf("got %d, want 123", got)
	}
}
