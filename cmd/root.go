/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)



// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "oss-triage-agent",
	Short: "AFK agent CLI for triaging, planning, and building GitHub issues",
	Long: `oss-triage-agent runs away-from-keyboard (AFK) coding agents against
open-source GitHub issues.

See README.md for CLI usage. Domain vocabulary and label contracts are in
CONTEXT.md; architecture and library docs are in docs/.`,
	Example: `  oss-triage-agent triage
  oss-triage-agent triage --issue 42
  oss-triage-agent build --issue 42 --repo .`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&issue, "issue", "i", "", "Issue number or URL")

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if _, err := resolveIssue(args); err == nil {
			return runTriage(cmd, args)
		}
		return cmd.Help()
	}
}


