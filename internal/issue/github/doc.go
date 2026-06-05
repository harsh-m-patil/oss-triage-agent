// Package github implements issue.IssueTracker against the GitHub REST API.
//
// Set GITHUB_TOKEN (repo scope, or issue read/write for public repos) before
// calling New. Tests inject an HTTP client and base URL via options; CI does
// not require a live token.
package github
