package triage

import "slices"

const LockLabel = "agent:in-progress"

var (
	categoryLabels = []string{"bug", "enhancement"}
	stateLabels    = []string{
		"needs-triage",
		"needs-info",
		"ready-for-agent",
		"ready-for-human",
		"wontfix",
	}
)

// RepoLabel is metadata for creating a label in the issue tracker.
type RepoLabel struct {
	Name        string
	Color       string
	Description string
}

// RequiredRepoLabels returns the canonical triage and lock labels for repo setup.
func RequiredRepoLabels() []RepoLabel {
	return []RepoLabel{
		{Name: "bug", Color: "d73a4a", Description: "Something isn't working"},
		{Name: "enhancement", Color: "a2eeef", Description: "New feature or request"},
		{Name: "needs-triage", Color: "fbca04", Description: "Awaiting initial triage assessment"},
		{Name: "needs-info", Color: "1d76db", Description: "Blocked on reporter or maintainer input"},
		{Name: "ready-for-agent", Color: "0e8a16", Description: "Approved for AFK agent workflows"},
		{Name: "ready-for-human", Color: "c5def5", Description: "Agent output needs maintainer review"},
		{Name: "wontfix", Color: "ffffff", Description: "Closed without implementation"},
		{Name: LockLabel, Color: "ededed", Description: "An agent run is in progress on this issue"},
	}
}

// IsCategoryLabel reports whether label is a triage category role.
func IsCategoryLabel(label string) bool {
	return slices.Contains(categoryLabels, label)
}

// IsStateLabel reports whether label is a triage state role.
func IsStateLabel(label string) bool {
	return slices.Contains(stateLabels, label)
}

// ValidCategory reports whether category is allowed in triage output.
func ValidCategory(category string) bool {
	return IsCategoryLabel(category)
}

// ValidState reports whether state is allowed in triage output.
func ValidState(state string) bool {
	return IsStateLabel(state)
}
