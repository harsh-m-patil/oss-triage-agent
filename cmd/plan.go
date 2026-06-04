/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// planCmd represents the plan command
var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: runPlan,
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
