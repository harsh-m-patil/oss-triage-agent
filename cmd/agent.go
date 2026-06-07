package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/harsh-m-patil/oss-triage-agent/internal/agent"
	"github.com/spf13/cobra"
)

var (
	agentProvider                   string
	agentModel                      string
	agentPrompt                     string
	agentVariant                    string
	agentName                       string
	agentThinking                   string
	agentSession                    string
	agentDangerouslySkipPermissions bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run and inspect AFK coding agents",
	Long:  "Low-level commands for running agent providers outside the triage, plan, and build workflows.",
}

var agentRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run an agent and print normalized stream events",
	Long: `Run an agent provider and emit normalized Agent events as JSON lines on stdout.

Useful for debugging stream parsing and agent configuration before wiring a
full workflow. Prompt via --prompt or stdin.`,
	Example: `  oss-triage-agent agent run --prompt "Summarize this repo"
  oss-triage-agent agent run --provider pi --model claude-sonnet-4 --prompt "Hi"
  echo "What changed?" | oss-triage-agent agent run`,
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentRunCmd)

	agentRunCmd.Flags().StringVar(&agentProvider, "provider", "opencode", "Agent provider (opencode or pi)")
	agentRunCmd.Flags().StringVar(&agentModel, "model", "opencode/big-pickle", "Model passed to the agent provider")
	agentRunCmd.Flags().StringVarP(&agentPrompt, "prompt", "p", "", "Prompt (reads stdin when empty)")
	agentRunCmd.Flags().StringVar(&agentVariant, "variant", "", "OpenCode --variant flag")
	agentRunCmd.Flags().StringVar(&agentName, "agent", "", "OpenCode --agent flag")
	agentRunCmd.Flags().StringVar(&agentThinking, "thinking", "", "Pi --thinking flag (off, minimal, low, medium, high, xhigh)")
	agentRunCmd.Flags().StringVar(&agentSession, "session", "", "Pi --session flag (resume session id)")
	agentRunCmd.Flags().BoolVar(&agentDangerouslySkipPermissions, "dangerously-skip-permissions", false, "OpenCode --dangerously-skip-permissions flag")
}

func runAgent(cmd *cobra.Command, args []string) error {
	provider, binary, err := resolveAgentProvider(agentProvider)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("%s binary not found on PATH: %w", binary, err)
	}

	prompt, err := resolveAgentPrompt(agentPrompt)
	if err != nil {
		return err
	}

	return streamAgentEvents(cmd.Context(), provider, prompt, os.Stdout, os.Stderr)
}

func resolveAgentProvider(name string) (agent.AgentProvider, string, error) {
	provider, err := resolveWorkflowAgent(workflowAgentConfig{
		Provider:                   name,
		Model:                      agentModel,
		Variant:                    agentVariant,
		AgentName:                  agentName,
		Thinking:                   agentThinking,
		Session:                    agentSession,
		DangerouslySkipPermissions: agentDangerouslySkipPermissions,
	})
	if err != nil {
		return nil, "", err
	}
	return provider, workflowAgentBinary(name), nil
}

func resolveAgentPrompt(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}
	data, err := io.ReadAll(bufio.NewReader(os.Stdin))
	if err != nil {
		return "", fmt.Errorf("read prompt from stdin: %w", err)
	}
	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return "", fmt.Errorf("prompt is required (use --prompt or pipe stdin)")
	}
	return prompt, nil
}

func streamAgentEvents(ctx context.Context, provider agent.AgentProvider, prompt string, out, errOut io.Writer) error {
	launch := provider.BuildLaunch(prompt)
	if len(launch.Argv) == 0 {
		return fmt.Errorf("provider %q returned empty command", provider.Name())
	}

	command := launch.Argv[0]
	args := launch.Argv[1:]

	execCmd := exec.CommandContext(ctx, command, args...)
	execCmd.Env = os.Environ()
	for k, v := range provider.Env() {
		execCmd.Env = append(execCmd.Env, k+"="+v)
	}
	if launch.Stdin != "" {
		execCmd.Stdin = strings.NewReader(launch.Stdin)
	}

	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := execCmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", command, err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var streamErr error
	recordStreamErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if streamErr == nil {
			streamErr = err
		}
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		recordStreamErr(streamStdoutEvents(stdout, provider, json.NewEncoder(out)))
	}()
	go func() {
		defer wg.Done()
		recordStreamErr(streamStderr(stderr, errOut))
	}()

	wg.Wait()
	waitErr := execCmd.Wait()
	if streamErr != nil {
		return errors.Join(streamErr, waitErr)
	}
	return waitErr
}

func streamStdoutEvents(r io.Reader, provider agent.AgentProvider, enc *json.Encoder) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			events, parseErr := provider.ParseStreamLine(line)
			if parseErr != nil {
				return parseErr
			}
			for _, event := range events {
				if err := enc.Encode(event); err != nil {
					return err
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func streamStderr(r io.Reader, out io.Writer) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if _, werr := out.Write([]byte(line)); werr != nil {
				return werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
