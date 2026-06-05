package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
	issuegithub "github.com/harsh-m-patil/oss-triage-agent/internal/issue/github"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
)

const defaultTriageIdleTimeout = 5 * time.Minute

var (
	triageRepoPath                   string
	triageModel                      string
	triageVariant                    string
	triageAgentName                  string
	triageSandboxMode                string
	triageDangerouslySkipPermissions bool
	triageIdleTimeout                time.Duration
)

func init() {
	triageCmd.Flags().StringVar(&triageRepoPath, "repo", ".", "Path to the target git repository root")
	triageCmd.Flags().StringVar(&triageModel, "model", "opencode/big-pickle", "Model passed to the agent provider")
	triageCmd.Flags().StringVar(&triageVariant, "variant", "", "OpenCode --variant flag")
	triageCmd.Flags().StringVar(&triageAgentName, "agent", "", "OpenCode --agent flag")
	triageCmd.Flags().StringVar(&triageSandboxMode, "sandbox", buildSandboxNoSandbox, "Sandbox mode: docker or nosandbox")
	triageCmd.Flags().BoolVar(&triageDangerouslySkipPermissions, "dangerously-skip-permissions", false, "OpenCode --dangerously-skip-permissions flag")
	triageCmd.Flags().DurationVar(&triageIdleTimeout, "idle-timeout", defaultTriageIdleTimeout, "Maximum idle time before the run is cancelled")
}

func resolveTriageWorkflowDeps(opts triageRuntimeOptions) (triageWorkflowDeps, error) {
	repoRoot, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return triageWorkflowDeps{}, fmt.Errorf("resolve repo path: %w", err)
	}
	owner, repo, err := githubRepoFromGitRemote(repoRoot)
	if err != nil {
		return triageWorkflowDeps{}, err
	}
	sandboxProvider, err := newBuildSandboxProvider(opts.SandboxMode)
	if err != nil {
		return triageWorkflowDeps{}, err
	}
	tracker, err := issuegithub.New(owner, repo)
	if err != nil {
		return triageWorkflowDeps{}, err
	}
	return triageWorkflowDeps{
		Issues:      tracker,
		Sandbox:     sandboxProvider,
		RepoRoot:    repoRoot,
		Agent: opencode.NewProvider(opts.Model, opencode.Options{
			Variant:                    opts.Variant,
			Agent:                      opts.AgentName,
			DangerouslySkipPermissions: opts.DangerouslySkipPermissions,
			Env:                        opencodeEnvFromOS(),
		}),
		Prompt: prompt.Builder{},
	}, nil
}
