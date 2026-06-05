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
open-source GitHub issues. Orchestration uses provider interfaces; concrete
backends (OpenCode, Docker, GitHub) live in adapters behind those seams.

Workflows:
  setup   Create required triage labels on the target GitHub repo
  triage  Assess issues, post triage comments, and apply category/state labels
  plan    Turn triaged issues into implementation plans (not implemented yet)
  build   Implement an issue in a sandboxed agent run with git worktrees

Pass --issue or a positional issue number/URL to target a GitHub issue.
When called with an issue id and no subcommand, triage runs by default.

Environment:
  GITHUB_TOKEN     GitHub API token for issue read/write (workflows)
  OPENCODE_API_KEY OpenCode API key when the configured model requires it

See README.md for CLI usage. Domain vocabulary and label contracts are in
CONTEXT.md.`,
	Example: `  # Triage one issue (same as the triage subcommand)
  oss-triage-agent --issue 42
  oss-triage-agent triage --issue 42

  # List issues needing triage
  oss-triage-agent triage

  # Run the build workflow
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


