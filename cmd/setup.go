/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	issuegithub "github.com/harsh-m-patil/oss-triage-agent/internal/issue/github"
	triagepkg "github.com/harsh-m-patil/oss-triage-agent/internal/triage"
	"github.com/spf13/cobra"
)

var setupRepoPath string

type repoLabelProvisioner interface {
	ListRepoLabels(ctx context.Context) ([]string, error)
	CreateRepoLabel(ctx context.Context, name, color, description string) error
}

var setupLabelProvisioner = func(repoPath string) (repoLabelProvisioner, string, error) {
	repoRoot, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve repo path: %w", err)
	}
	owner, repo, err := githubRepoFromGitRemote(repoRoot)
	if err != nil {
		return nil, "", err
	}
	tracker, err := issuegithub.New(owner, repo)
	if err != nil {
		return nil, "", err
	}
	return tracker, fmt.Sprintf("%s/%s", owner, repo), nil
}

// setupCmd creates required triage labels on the target GitHub repository.
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Create required triage labels on the target GitHub repo",
	Long: `Ensure the canonical triage and agent-lock labels exist on the GitHub
repository resolved from remote.origin.url in --repo.

Creates any missing labels and skips labels that already exist. Requires
GITHUB_TOKEN with permission to manage labels on the target repo.`,
	Example: `  oss-triage-agent setup --repo .
  oss-triage-agent setup --repo /path/to/target-repo`,
	RunE: runSetup,
}

func runSetup(cmd *cobra.Command, args []string) error {
	provisioner, repoSlug, err := setupLabelProvisioner(setupRepoPath)
	if err != nil {
		return err
	}

	created, skipped, err := ensureRepoLabels(cmd.Context(), provisioner)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if len(created) == 0 {
		_, _ = fmt.Fprintf(out, "All %d required labels already exist on %s.\n", len(skipped), repoSlug)
		return nil
	}

	_, _ = fmt.Fprintf(out, "Created %d label(s) on %s:\n", len(created), repoSlug)
	for _, name := range created {
		_, _ = fmt.Fprintf(out, "  + %s\n", name)
	}
	if len(skipped) > 0 {
		_, _ = fmt.Fprintf(out, "Skipped %d existing label(s).\n", len(skipped))
	}
	return nil
}

func ensureRepoLabels(ctx context.Context, provisioner repoLabelProvisioner) (created, skipped []string, err error) {
	existing, err := provisioner.ListRepoLabels(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list repo labels: %w", err)
	}

	for _, spec := range triagepkg.RequiredRepoLabels() {
		if slices.Contains(existing, spec.Name) {
			skipped = append(skipped, spec.Name)
			continue
		}
		if err := provisioner.CreateRepoLabel(ctx, spec.Name, spec.Color, spec.Description); err != nil {
			return created, skipped, fmt.Errorf("create label %q: %w", spec.Name, err)
		}
		created = append(created, spec.Name)
	}
	return created, skipped, nil
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVar(&setupRepoPath, "repo", ".", "Path to the target git repository root")
}
