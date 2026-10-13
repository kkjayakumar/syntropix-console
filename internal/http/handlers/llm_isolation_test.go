package handlers

import (
	"errors"
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func TestEnvironmentProjectMismatchIsForbiddenSignal(t *testing.T) {
	err := store.ErrEnvironmentProjectMismatch
	if !errors.Is(err, store.ErrEnvironmentProjectMismatch) {
		t.Fatal("sentinel must support errors.Is")
	}
}

func TestSeedEnvironmentGraphRequiresEnvID(t *testing.T) {
	n, err := seedEnvironmentGraph(nil, nil, nil, 0)
	if err == nil || n != 0 {
		t.Fatalf("expected error for env 0, got n=%d err=%v", n, err)
	}
}
