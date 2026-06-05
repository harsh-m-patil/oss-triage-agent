package cmd

import (
	"context"
	"fmt"
	"slices"
	"testing"

	triagepkg "github.com/harsh-m-patil/oss-triage-agent/internal/triage"
)

type fakeLabelProvisioner struct {
	existing []string
	created  []string
	failOn   string
}

func (f *fakeLabelProvisioner) ListRepoLabels(context.Context) ([]string, error) {
	return append([]string(nil), f.existing...), nil
}

func (f *fakeLabelProvisioner) CreateRepoLabel(_ context.Context, name, color, description string) error {
	if name == f.failOn {
		return fmt.Errorf("create failed")
	}
	if color == "" || description == "" {
		return fmt.Errorf("missing metadata for %q", name)
	}
	f.created = append(f.created, name)
	f.existing = append(f.existing, name)
	return nil
}

func TestEnsureRepoLabels_createsMissingLabels(t *testing.T) {
	t.Parallel()

	provisioner := &fakeLabelProvisioner{existing: []string{"bug"}}
	created, skipped, err := ensureRepoLabels(context.Background(), provisioner)
	if err != nil {
		t.Fatalf("ensureRepoLabels: %v", err)
	}

	wantCreated := len(triagepkg.RequiredRepoLabels()) - 1
	if len(created) != wantCreated {
		t.Fatalf("created = %v (%d), want %d labels", created, len(created), wantCreated)
	}
	if !slices.Contains(skipped, "bug") {
		t.Fatalf("skipped = %v, want bug", skipped)
	}
	if slices.Contains(created, "bug") {
		t.Fatalf("created should not include existing bug label: %v", created)
	}
}

func TestEnsureRepoLabels_skipsWhenAllExist(t *testing.T) {
	t.Parallel()

	existing := make([]string, 0, len(triagepkg.RequiredRepoLabels()))
	for _, spec := range triagepkg.RequiredRepoLabels() {
		existing = append(existing, spec.Name)
	}
	provisioner := &fakeLabelProvisioner{existing: existing}

	created, skipped, err := ensureRepoLabels(context.Background(), provisioner)
	if err != nil {
		t.Fatalf("ensureRepoLabels: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("created = %v, want none", created)
	}
	if len(skipped) != len(triagepkg.RequiredRepoLabels()) {
		t.Fatalf("skipped = %d, want %d", len(skipped), len(triagepkg.RequiredRepoLabels()))
	}
}

func TestEnsureRepoLabels_returnsCreateError(t *testing.T) {
	t.Parallel()

	provisioner := &fakeLabelProvisioner{failOn: "needs-triage"}
	_, _, err := ensureRepoLabels(context.Background(), provisioner)
	if err == nil {
		t.Fatal("ensureRepoLabels: expected error")
	}
}
