package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/db"
)

func TestAssertNodesBelongToEnvironment(t *testing.T) {
	nodes := []GraphNode{
		{ID: 1, EnvironmentID: 10, Title: "a"},
		{ID: 2, EnvironmentID: 10, Title: "b"},
	}
	if err := assertNodesBelongToEnvironment(nodes, 10); err != nil {
		t.Fatalf("same env should pass: %v", err)
	}
	nodes = append(nodes, GraphNode{ID: 3, EnvironmentID: 99, Title: "leak"})
	if err := assertNodesBelongToEnvironment(nodes, 10); !errors.Is(err, ErrGraphIsolation) {
		t.Fatalf("expected ErrGraphIsolation, got %v", err)
	}
}

func TestAssertEdgesBelongToEnvironment(t *testing.T) {
	edges := []GraphEdge{{ID: 1, EnvironmentID: 5}}
	if err := assertEdgesBelongToEnvironment(edges, 5); err != nil {
		t.Fatal(err)
	}
	if err := assertEdgesBelongToEnvironment([]GraphEdge{{ID: 2, EnvironmentID: 6}}, 5); !errors.Is(err, ErrGraphIsolation) {
		t.Fatalf("expected isolation error, got %v", err)
	}
}

func TestGraphIsolationEnvANeverReturnsEnvB(t *testing.T) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if os.Getenv("MIGRATIONS_DIR") == "" {
		for _, candidate := range []string{"migrations", filepath.Join("..", "..", "migrations")} {
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				_ = os.Setenv("MIGRATIONS_DIR", candidate)
				break
			}
		}
	}
	if err := db.RunMigrations(ctx, url); err != nil {
		t.Skipf("migrations unavailable: %v", err)
	}
	database, err := db.Connect(ctx, url)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	defer database.Close()

	ws := NewWorkspaceStore(database.Pool)
	graph := NewGraphStore(database.Pool)

	boot, err := ws.Bootstrap(ctx, "syntropix")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	suffix := time.Now().UnixNano()
	pA, err := ws.CreateProject(ctx, boot.Tenant.Slug, fmt.Sprintf("iso-a-%d", suffix), "Isolation A", "compliance test")
	if err != nil {
		t.Fatalf("create project A: %v", err)
	}
	pB, err := ws.CreateProject(ctx, boot.Tenant.Slug, fmt.Sprintf("iso-b-%d", suffix), "Isolation B", "compliance test")
	if err != nil {
		t.Fatalf("create project B: %v", err)
	}
	envA, err := ws.CreateEnvironment(ctx, pA.ID, "dev", "Dev A", 1)
	if err != nil {
		t.Fatalf("env A: %v", err)
	}
	envB, err := ws.CreateEnvironment(ctx, pB.ID, "dev", "Dev B", 1)
	if err != nil {
		t.Fatalf("env B: %v", err)
	}

	t.Cleanup(func() {
		_, _ = database.Pool.Exec(context.Background(), `DELETE FROM aegis_projects WHERE id IN ($1, $2)`, pA.ID, pB.ID)
	})

	if err := ws.AssertEnvironmentBelongsToProject(ctx, envA.ID, pA.ID); err != nil {
		t.Fatalf("env A should belong to project A: %v", err)
	}
	if err := ws.AssertEnvironmentBelongsToProject(ctx, envA.ID, pB.ID); !errors.Is(err, ErrEnvironmentProjectMismatch) {
		t.Fatalf("expected project mismatch, got %v", err)
	}

	nodeA, err := graph.UpsertNode(ctx, envA.ID, "service", "secret-a", "Tenant A Secret", "PHI/finance data for A only", nil)
	if err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	nodeB, err := graph.UpsertNode(ctx, envB.ID, "service", "secret-b", "Tenant B Secret", "PHI/finance data for B only", nil)
	if err != nil {
		t.Fatalf("upsert B: %v", err)
	}
	if _, err := graph.UpsertEdge(ctx, envA.ID, nodeA.ID, nodeB.ID, "leaks"); !errors.Is(err, ErrGraphIsolation) {
		t.Fatalf("cross-env edge must be rejected, got %v", err)
	}

	snapA, err := graph.Snapshot(ctx, envA.ID)
	if err != nil {
		t.Fatalf("snapshot A: %v", err)
	}
	for _, n := range snapA.Nodes {
		if n.EnvironmentID != envA.ID {
			t.Fatalf("snapshot A leaked env %d node %q", n.EnvironmentID, n.Title)
		}
		if n.NodeKey == "secret-b" || n.Title == "Tenant B Secret" {
			t.Fatalf("env A snapshot contains env B node")
		}
	}

	sub, err := graph.SubgraphForQuery(ctx, envA.ID, "Tenant B Secret PHI finance", 20)
	if err != nil {
		t.Fatalf("subgraph: %v", err)
	}
	for _, n := range sub {
		if n.EnvironmentID != envA.ID {
			t.Fatalf("subgraph leaked env %d", n.EnvironmentID)
		}
		if n.NodeKey == "secret-b" {
			t.Fatal("SubgraphForQuery returned env B node for env A query")
		}
	}

	subB, err := graph.SubgraphForQuery(ctx, envB.ID, "Tenant A Secret", 20)
	if err != nil {
		t.Fatalf("subgraph B: %v", err)
	}
	for _, n := range subB {
		if n.NodeKey == "secret-a" {
			t.Fatal("env B subgraph returned env A node")
		}
	}
}
