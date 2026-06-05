package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

const (
	defaultBaseURL = "https://api.github.com"
	tokenEnvVar    = "GITHUB_TOKEN"
	lockLabel      = "agent:in-progress"
	listIssuesCap  = 100
)

var issueURLPattern = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/issues/(\d+)`)

var _ issue.IssueTracker = (*Tracker)(nil)

// Tracker implements issue.IssueTracker against the GitHub REST API.
type Tracker struct {
	owner   string
	repo    string
	client  *http.Client
	baseURL string
	token   string
}

// Option configures a Tracker.
type Option func(*Tracker)

// WithHTTPClient sets the HTTP client (for httptest injection).
func WithHTTPClient(c *http.Client) Option {
	return func(t *Tracker) {
		t.client = c
	}
}

// WithBaseURL overrides the GitHub API base URL (for httptest).
func WithBaseURL(url string) Option {
	return func(t *Tracker) {
		t.baseURL = strings.TrimRight(url, "/")
	}
}

// WithToken sets the API token explicitly instead of reading GITHUB_TOKEN.
func WithToken(token string) Option {
	return func(t *Tracker) {
		t.token = token
	}
}

// New returns a GitHub issue tracker for owner/repo.
// Authentication uses GITHUB_TOKEN unless overridden with WithToken.
func New(owner, repo string, opts ...Option) (*Tracker, error) {
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}

	t := &Tracker{
		owner:   owner,
		repo:    repo,
		client:  http.DefaultClient,
		baseURL: defaultBaseURL,
		token:   os.Getenv(tokenEnvVar),
	}
	for _, opt := range opts {
		opt(t)
	}
	if t.token == "" {
		return nil, fmt.Errorf("%s is not set", tokenEnvVar)
	}
	return t, nil
}

type ghIssue struct {
	Number int        `json:"number"`
	Title  string     `json:"title"`
	Body   string     `json:"body"`
	Labels []ghLabel  `json:"labels"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghSearchResult struct {
	Items []ghIssue `json:"items"`
}

func (t *Tracker) ReadIssue(ctx context.Context, id string) (*issue.Issue, error) {
	num, err := resolveIssueNumber(t.owner, t.repo, id)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/repos/%s/%s/issues/%d", t.owner, t.repo, num)
	var raw ghIssue
	if err := t.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	return normalizeIssue(raw), nil
}

func (t *Tracker) ListIssues(ctx context.Context, query string) ([]issue.Issue, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		path := fmt.Sprintf("/repos/%s/%s/issues?state=open&per_page=%d", t.owner, t.repo, listIssuesCap)
		var raw []ghIssue
		if err := t.getJSON(ctx, path, &raw); err != nil {
			return nil, err
		}
		return normalizeIssues(raw), nil
	}

	label, ok := parseLabelQuery(query)
	if !ok {
		return nil, fmt.Errorf("unsupported list query %q", query)
	}

	searchQ := fmt.Sprintf("repo:%s/%s is:issue is:open label:%s", t.owner, t.repo, label)
	path := "/search/issues?q=" + url.QueryEscape(searchQ) + "&per_page=" + strconv.Itoa(listIssuesCap)
	var result ghSearchResult
	if err := t.getJSON(ctx, path, &result); err != nil {
		return nil, err
	}
	return normalizeIssues(result.Items), nil
}

func (t *Tracker) Comment(ctx context.Context, id, body string) error {
	num, err := resolveIssueNumber(t.owner, t.repo, id)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", t.owner, t.repo, num)
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return err
	}
	return t.postJSON(ctx, path, payload, nil)
}

func (t *Tracker) AddLabel(ctx context.Context, id, label string) error {
	num, err := resolveIssueNumber(t.owner, t.repo, id)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", t.owner, t.repo, num)
	payload, err := json.Marshal([]string{label})
	if err != nil {
		return err
	}
	return t.postJSON(ctx, path, payload, nil)
}

func (t *Tracker) RemoveLabel(ctx context.Context, id, label string) error {
	num, err := resolveIssueNumber(t.owner, t.repo, id)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels/%s", t.owner, t.repo, num, url.PathEscape(label))
	return t.delete(ctx, path)
}

// Lock adds the agent:in-progress lock label.
func (t *Tracker) Lock(ctx context.Context, id string) error {
	return t.AddLabel(ctx, id, lockLabel)
}

// Unlock removes only the agent:in-progress lock label.
func (t *Tracker) Unlock(ctx context.Context, id string) error {
	return t.RemoveLabel(ctx, id, lockLabel)
}

func (t *Tracker) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL+path, nil)
	if err != nil {
		return err
	}
	t.setHeaders(req)
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, dest)
}

func (t *Tracker) postJSON(ctx context.Context, path string, payload []byte, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	t.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if dest == nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return apiError(resp)
		}
		return nil
	}
	return decodeResponse(resp, dest)
}

func (t *Tracker) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, t.baseURL+path, nil)
	if err != nil {
		return err
	}
	t.setHeaders(req)
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}
	return nil
}

func (t *Tracker) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func decodeResponse(resp *http.Response, dest any) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dest)
}

func apiError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return fmt.Errorf("github api: %s", resp.Status)
	}
	return fmt.Errorf("github api: %s: %s", resp.Status, msg)
}

func resolveIssueNumber(owner, repo, id string) (int, error) {
	if n, err := strconv.Atoi(strings.TrimSpace(id)); err == nil && strings.TrimSpace(id) != "" {
		return n, nil
	}
	matches := issueURLPattern.FindStringSubmatch(id)
	if matches == nil {
		return 0, fmt.Errorf("invalid issue id %q", id)
	}
	if matches[1] != owner || matches[2] != repo {
		return 0, fmt.Errorf("issue URL repo %s/%s does not match configured %s/%s", matches[1], matches[2], owner, repo)
	}
	return strconv.Atoi(matches[3])
}

func parseLabelQuery(query string) (string, bool) {
	const prefix = "label:"
	if !strings.HasPrefix(query, prefix) {
		return "", false
	}
	label := strings.TrimSpace(strings.TrimPrefix(query, prefix))
	if label == "" {
		return "", false
	}
	return label, true
}

func normalizeIssue(raw ghIssue) *issue.Issue {
	it := issue.Issue{
		Number: raw.Number,
		Title:  raw.Title,
		Body:   raw.Body,
	}
	for _, l := range raw.Labels {
		it.Labels = append(it.Labels, l.Name)
	}
	return &it
}

func normalizeIssues(raw []ghIssue) []issue.Issue {
	out := make([]issue.Issue, 0, len(raw))
	for _, r := range raw {
		out = append(out, *normalizeIssue(r))
	}
	return out
}

