package orchestrator

import (
	"context"
	"strings"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func deploymentForBundle(ctx context.Context, deployments *store.DeploymentStore, correlationID string) *store.Deployment {
	deps := RecentDeploymentsForIssue(ctx, deployments, correlationID, 1)
	if len(deps) == 0 {
		return nil
	}
	return &deps[0]
}

// RecentDeploymentsForIssue returns CI runs linked to an issue (ci_* correlation only).
func RecentDeploymentsForIssue(ctx context.Context, deployments *store.DeploymentStore, correlationID string, limit int) []store.Deployment {
	if deployments == nil || !IssueShowsCICD(correlationID) {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	all, err := deployments.ListRecent(ctx, 50)
	if err != nil || len(all) == 0 {
		return nil
	}
	runID := strings.TrimPrefix(correlationID, "ci_")
	for i := range all {
		if all[i].RunID == runID && !IsSyntheticDeployment(all[i]) {
			return []store.Deployment{all[i]}
		}
	}
	return nil
}
