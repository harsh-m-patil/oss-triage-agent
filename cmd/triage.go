/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// triageCmd represents the triage command
var triageCmd = &cobra.Command{
	Use:   "triage",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: runTriage,
}

func runTriage(cmd *cobra.Command, args []string) error {
	issueID, err := resolveIssue(args)
	if err != nil {
		return err
	}

	fmt.Printf("triaging issue %s\n", issueID)
	return nil
}

func init() {
	rootCmd.AddCommand(triageCmd)
}
