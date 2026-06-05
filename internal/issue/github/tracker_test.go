package github_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
	"github.com/harsh-m-patil/oss-triage-agent/internal/issue/github"
)

func TestNew_requiresToken(t *testing.T) {
	t.Parallel()

	_, err := github.New("acme", "widget", github.WithToken(""))
	if err == nil {
		t.Fatal("New: expected error when token is empty")
	}
}

func newTestTracker(t *testing.T, handler http.HandlerFunc) *github.Tracker {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	tracker, err := github.New("acme", "widget",
		github.WithBaseURL(srv.URL),
		github.WithHTTPClient(srv.Client()),
		github.WithToken("test-token"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return tracker
}

func ghIssueJSON(number int, title, body string, labels ...string) string {
	type label struct {
		Name string `json:"name"`
	}
	issue := struct {
		Number int     `json:"number"`
		Title  string  `json:"title"`
		Body   string  `json:"body"`
		Labels []label `json:"labels"`
	}{
		Number: number,
		Title:  title,
		Body:   body,
	}
	for _, l := range labels {
		issue.Labels = append(issue.Labels, label{Name: l})
	}
	b, err := json.Marshal(issue)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestTracker_ReadIssue_byNumericID_returnsNormalizedIssue(t *testing.T) {
	t.Parallel()

	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/widget/issues/42" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, ghIssueJSON(42, "fix login", "details", "bug", "needs-triage"))
	})

	got, err := tracker.ReadIssue(context.Background(), "42")
	if err != nil {
		t.Fatalf("ReadIssue: %v", err)
	}
	want := &issue.Issue{
		Number: 42,
		Title:  "fix login",
		Body:   "details",
		Labels: []string{"bug", "needs-triage"},
	}
	if got.Number != want.Number || got.Title != want.Title || got.Body != want.Body {
		t.Fatalf("ReadIssue = %+v, want %+v", got, want)
	}
	if len(got.Labels) != len(want.Labels) || got.Labels[0] != want.Labels[0] || got.Labels[1] != want.Labels[1] {
		t.Fatalf("Labels = %v, want %v", got.Labels, want.Labels)
	}
}

func TestTracker_ReadIssue_byIssueURL_returnsNormalizedIssue(t *testing.T) {
	t.Parallel()

	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/widget/issues/7" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, ghIssueJSON(7, "from url", "body"))
	})

	got, err := tracker.ReadIssue(context.Background(), "https://github.com/acme/widget/issues/7")
	if err != nil {
		t.Fatalf("ReadIssue: %v", err)
	}
	if got.Number != 7 || got.Title != "from url" {
		t.Fatalf("ReadIssue = %+v, want issue 7", got)
	}
}

func TestTracker_Comment_postsBodyToIssue(t *testing.T) {
	t.Parallel()

	var gotBody string
	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/issues/3/comments" {
			http.NotFound(w, r)
			return
		}
		var payload struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode comment body: %v", err)
		}
		gotBody = payload.Body
		w.WriteHeader(http.StatusCreated)
	})

	if err := tracker.Comment(context.Background(), "3", "triage complete"); err != nil {
		t.Fatalf("Comment: %v", err)
	}
	if gotBody != "triage complete" {
		t.Fatalf("comment body = %q, want %q", gotBody, "triage complete")
	}
}

func TestTracker_AddLabel_addsSingleLabelWithoutDroppingOthers(t *testing.T) {
	t.Parallel()

	var gotLabels []string
	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/issues/5/labels" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotLabels); err != nil {
			t.Errorf("decode labels: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := tracker.AddLabel(context.Background(), "5", "ready-for-agent"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	if len(gotLabels) != 1 || gotLabels[0] != "ready-for-agent" {
		t.Fatalf("posted labels = %v, want [ready-for-agent]", gotLabels)
	}
}

func TestTracker_RemoveLabel_removesSingleLabel(t *testing.T) {
	t.Parallel()

	var gotPath string
	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	if err := tracker.RemoveLabel(context.Background(), "9", "needs-info"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	want := "/repos/acme/widget/issues/9/labels/needs-info"
	if gotPath != want {
		t.Fatalf("DELETE path = %q, want %q", gotPath, want)
	}
}

func TestTracker_Lock_addsAgentInProgressLabel(t *testing.T) {
	t.Parallel()

	var gotLabels []string
	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/issues/11/labels" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&gotLabels)
		w.WriteHeader(http.StatusOK)
	})

	if err := tracker.Lock(context.Background(), "11"); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if len(gotLabels) != 1 || gotLabels[0] != "agent:in-progress" {
		t.Fatalf("Lock labels = %v, want [agent:in-progress]", gotLabels)
	}
}

func TestTracker_Unlock_removesOnlyAgentInProgressLabel(t *testing.T) {
	t.Parallel()

	var gotPath string
	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.NotFound(w, r)
			return
		}
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	if err := tracker.Unlock(context.Background(), "12"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	want := "/repos/acme/widget/issues/12/labels/agent:in-progress"
	if gotPath != want {
		t.Fatalf("DELETE path = %q, want %q", gotPath, want)
	}
}

func TestTracker_ListIssues_withoutQuery_returnsOpenIssues(t *testing.T) {
	t.Parallel()

	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/widget/issues" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("state") != "open" {
			t.Errorf("state = %q, want open", r.URL.Query().Get("state"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "["+ghIssueJSON(1, "one", "")+","+ghIssueJSON(2, "two", "")+"]")
	})

	got, err := tracker.ListIssues(context.Background(), "")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(got) != 2 || got[0].Number != 1 || got[1].Number != 2 {
		t.Fatalf("ListIssues = %+v, want issues 1 and 2", got)
	}
}

func TestTracker_ListRepoLabels_returnsLabelNames(t *testing.T) {
	t.Parallel()

	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/widget/labels" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"name":"bug"},{"name":"needs-triage"}]`)
	})

	got, err := tracker.ListRepoLabels(context.Background())
	if err != nil {
		t.Fatalf("ListRepoLabels: %v", err)
	}
	if len(got) != 2 || got[0] != "bug" || got[1] != "needs-triage" {
		t.Fatalf("ListRepoLabels = %v, want [bug needs-triage]", got)
	}
}

func TestTracker_CreateRepoLabel_postsMetadata(t *testing.T) {
	t.Parallel()

	var got map[string]string
	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/repos/acme/widget/labels" {
			http.NotFound(w, r)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	})

	if err := tracker.CreateRepoLabel(context.Background(), "ready-for-agent", "0e8a16", "Approved for AFK agent workflows"); err != nil {
		t.Fatalf("CreateRepoLabel: %v", err)
	}
	if got["name"] != "ready-for-agent" || got["color"] != "0e8a16" || got["description"] != "Approved for AFK agent workflows" {
		t.Fatalf("CreateRepoLabel payload = %#v", got)
	}
}

func TestTracker_ListIssues_withLabelFilter_usesSearchAPI(t *testing.T) {
	t.Parallel()

	tracker := newTestTracker(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/search/issues" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "label:needs-triage") || !strings.Contains(q, "repo:acme/widget") {
			t.Errorf("search q = %q, want repo and label filter", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[`+ghIssueJSON(4, "triage me", "", "needs-triage")+`]}`)
	})

	got, err := tracker.ListIssues(context.Background(), "label:needs-triage")
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(got) != 1 || got[0].Number != 4 {
		t.Fatalf("ListIssues = %+v, want issue 4", got)
	}
}
