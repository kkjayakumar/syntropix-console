package orchestrator

import (
	"errors"
	"testing"

	githubclient "github.com/kkjayakumar/AegIsOps/internal/githubclient"
)

func TestShouldRetryGitHubIssueWithoutLabels(t *testing.T) {
	in := githubclient.CreateIssueInput{Labels: []string{"aegisops-detected"}}
	if !shouldRetryGitHubIssueWithoutLabels(errors.New("github create issue 422: Validation Failed"), in.Labels) {
		t.Fatal("expected retry on 422")
	}
	if shouldRetryGitHubIssueWithoutLabels(nil, in.Labels) {
		t.Fatal("no retry without error")
	}
	if shouldRetryGitHubIssueWithoutLabels(errors.New("403"), in.Labels) {
		t.Fatal("no retry on non-label errors")
	}
}
