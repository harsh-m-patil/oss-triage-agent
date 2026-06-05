/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Run the AFK plan workflow for an issue (not implemented yet)",
	Long: `Turn a triaged GitHub issue into an implementation plan by running an
AFK agent over the issue body and repository context.

This subcommand is a stub. Use triage to assess issues and build to implement them.`,
	Example: `  oss-triage-agent plan --issue 42`,
	RunE:    runPlan,
}

func runPlan(cmd *cobra.Command, args []string) error {
	issueID, err := resolveIssue(args)
	if err != nil {
		return err
	}

	fmt.Printf("planning issue %s\n", issueID)
	return nil
}

func init() {
	rootCmd.AddCommand(planCmd)
}
