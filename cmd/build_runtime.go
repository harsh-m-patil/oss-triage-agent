package cmd

import (
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/harsh-m-patil/oss-triage-agent/internal/git/local"
	issuegithub "github.com/harsh-m-patil/oss-triage-agent/internal/issue/github"
	"github.com/harsh-m-patil/oss-triage-agent/internal/prompt"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox"
	dockersandbox "github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/docker"
	"github.com/harsh-m-patil/oss-triage-agent/internal/sandbox/nosandbox"
)

const (
	buildSandboxDocker    = "docker"
	buildSandboxNoSandbox = "nosandbox"
	defaultBuildIdleTimeout = 5 * time.Minute
)

var (
	buildRepoPath                   string
	buildProvider                   string
	buildModel                      string
	buildVariant                    string
	buildAgentName                  string
	buildThinking                   string
	buildSession                    string
	buildSandboxMode                string
	buildDangerouslySkipPermissions bool
	buildIdleTimeout                time.Duration
)

func init() {
	buildCmd.Flags().StringVar(&buildRepoPath, "repo", ".", "Path to the target git repository root")
	buildCmd.Flags().StringVar(&buildProvider, "provider", workflowProviderOpenCode, "Agent provider (opencode or pi)")
	buildCmd.Flags().StringVar(&buildModel, "model", "opencode/big-pickle", "Model passed to the agent provider")
	buildCmd.Flags().StringVar(&buildVariant, "variant", "", "OpenCode --variant flag")
	buildCmd.Flags().StringVar(&buildAgentName, "agent", "", "OpenCode --agent flag")
	buildCmd.Flags().StringVar(&buildThinking, "thinking", "", "Pi --thinking flag (off, minimal, low, medium, high, xhigh)")
	buildCmd.Flags().StringVar(&buildSession, "session", "", "Pi --session flag (resume session id)")
	buildCmd.Flags().StringVar(&buildSandboxMode, "sandbox", buildSandboxDocker, "Sandbox mode: docker or nosandbox")
	buildCmd.Flags().BoolVar(&buildDangerouslySkipPermissions, "dangerously-skip-permissions", false, "OpenCode --dangerously-skip-permissions flag")
	buildCmd.Flags().DurationVar(&buildIdleTimeout, "idle-timeout", defaultBuildIdleTimeout, "Maximum idle time before the run is cancelled")
}

func resolveBuildWorkflowDeps(opts buildRuntimeOptions) (buildWorkflowDeps, error) {
	repoRoot, err := filepath.Abs(opts.RepoPath)
	if err != nil {
		return buildWorkflowDeps{}, fmt.Errorf("resolve repo path: %w", err)
	}
	owner, repo, err := githubRepoFromGitRemote(repoRoot)
	if err != nil {
		return buildWorkflowDeps{}, err
	}
	sandboxProvider, err := newBuildSandboxProvider(opts.SandboxMode)
	if err != nil {
		return buildWorkflowDeps{}, err
	}
	tracker, err := issuegithub.New(owner, repo)
	if err != nil {
		return buildWorkflowDeps{}, err
	}
	agentProvider, err := resolveWorkflowAgent(workflowAgentConfig{
		Provider:                   opts.Provider,
		Model:                      opts.Model,
		Variant:                    opts.Variant,
		AgentName:                  opts.AgentName,
		Thinking:                   opts.Thinking,
		Session:                    opts.Session,
		DangerouslySkipPermissions: opts.DangerouslySkipPermissions,
	})
	if err != nil {
		return buildWorkflowDeps{}, err
	}

	return buildWorkflowDeps{
		Issues: tracker,
		Repo:   local.New(repoRoot),
		Sandbox: sandboxProvider,
		Agent:   agentProvider,
		Prompt:  prompt.Builder{},
	}, nil
}

func newBuildSandboxProvider(mode string) (sandbox.SandboxProvider, error) {
	switch strings.TrimSpace(mode) {
	case "", buildSandboxDocker:
		return dockersandbox.NewProvider()
	case buildSandboxNoSandbox:
		return nosandbox.NewProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported sandbox mode %q (want %q or %q)", mode, buildSandboxDocker, buildSandboxNoSandbox)
	}
}

func githubRepoFromGitRemote(repoRoot string) (string, string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("read remote.origin.url: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseGitHubRepoRemote(strings.TrimSpace(string(out)))
}

func parseGitHubRepoRemote(raw string) (string, string, error) {
	if strings.HasPrefix(raw, "git@github.com:") {
		return splitGitHubPath(strings.TrimPrefix(raw, "git@github.com:"))
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("parse github remote %q: %w", raw, err)
	}
	if u.Host != "github.com" && u.Host != "www.github.com" {
		return "", "", fmt.Errorf("unsupported git remote host %q", u.Host)
	}
	return splitGitHubPath(strings.TrimPrefix(u.Path, "/"))
}

func splitGitHubPath(path string) (string, string, error) {
	path = strings.TrimSuffix(strings.TrimSpace(path), ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("unsupported github remote path %q", path)
	}
	return parts[0], parts[1], nil
}
