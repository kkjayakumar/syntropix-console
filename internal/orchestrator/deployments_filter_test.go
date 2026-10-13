package orchestrator

import (
	"context"
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func TestIsSyntheticDeployment(t *testing.T) {
	cases := []struct {
		name string
		dep  store.Deployment
		want bool
	}{
		{"smoke sha", store.Deployment{RunID: "123456789", CommitSHA: "abc1234", Branch: "main"}, true},
		{"verify script run", store.Deployment{RunID: "999001", CommitSHA: "abc1234", Branch: "main"}, true},
		{"phase3 deploy", store.Deployment{RunID: "555555555", CommitSHA: "abc123def456", Branch: "main"}, true},
		{"staging smoke sha", store.Deployment{RunID: "888888888", CommitSHA: "staging_smoke", Branch: "main"}, true},
		{"real looking", store.Deployment{RunID: "1234567890", CommitSHA: "34bf3f5123ab", Branch: "main"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSyntheticDeployment(tc.dep); got != tc.want {
				t.Fatalf("IsSyntheticDeployment() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIssueShowsCICD(t *testing.T) {
	if IssueShowsCICD("feat_manual_123") {
		t.Fatal("manual feat should not show CI/CD")
	}
	if !IssueShowsCICD("ci_1234567890") {
		t.Fatal("ci correlation should show CI/CD")
	}
}

func TestRecentDeploymentsForIssueManualFeat(t *testing.T) {
	got := RecentDeploymentsForIssue(context.Background(), nil, "feat_manual_1781966984", 5)
	if got != nil {
		t.Fatalf("expected nil for feat_manual, got %v", got)
	}
}
