package triage

import "slices"

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
