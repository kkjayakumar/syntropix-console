package orchestrator

import "strings"

func githubIssueLabels(correlationID string) []string {
	labels := []string{"aegisops-detected", "bug"}
	if strings.HasPrefix(correlationID, "req_aegis_") {
		labels = append(labels, "aegisops-selftest")
	}
	return labels
}
