package issue_test

import (
	"context"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue/fake"
)

func TestFakeTracker_ReadIssue_returnsStoredIssue(t *testing.T) {
	t.Parallel()

	tracker := fake.NewTracker(map[string]fake.Issue{
		"42": {Number: 42, Title: "setup internal packages", Body: "brief"},
	})

	got, err := tracker.ReadIssue(context.Background(), "42")
	if err != nil {
		t.Fatalf("ReadIssue: %v", err)
	}
	if got.Number != 42 || got.Title != "setup internal packages" {
		t.Fatalf("ReadIssue = %+v, want number 42", got)
	}
}
