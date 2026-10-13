package orchestrator

import (
	"strings"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

// IsSyntheticDeployment detects rows created by local/staging smoke scripts, not real GitHub Actions.
func IsSyntheticDeployment(d store.Deployment) bool {
	sha := strings.ToLower(strings.TrimSpace(d.CommitSHA))
	runID := strings.TrimSpace(d.RunID)

	switch sha {
	case "abc1234", "abc123def456", "staging", "staging_smoke":
		return true
	}
	if strings.HasPrefix(sha, "abc123") {
		return true
	}
	if runID == "999001" {
		return true
	}
	// Real GitHub Actions run IDs are large integers (typically 9+ digits).
	if runID != "" && len(runID) < 8 && isDigits(runID) {
		return true
	}
	return false
}

func FilterRealDeployments(deps []store.Deployment) []store.Deployment {
	if len(deps) == 0 {
		return nil
	}
	out := make([]store.Deployment, 0, len(deps))
	for _, d := range deps {
		if !IsSyntheticDeployment(d) {
			out = append(out, d)
		}
	}
	return out
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// IssueShowsCICD returns true when CI/CD context belongs on the issue inspector.
func IssueShowsCICD(correlationID string) bool {
	return strings.HasPrefix(correlationID, "ci_")
}
