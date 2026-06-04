package cmd

import "fmt"

var issue string

func resolveIssue(args []string) (string, error) {
	issueID := issue
	if issueID == "" && len(args) > 0 {
		issueID = args[0]
	}
	if issueID == "" {
		return "", fmt.Errorf("issue required: pass --issue or provide an issue number argument")
	}
	return issueID, nil
}
