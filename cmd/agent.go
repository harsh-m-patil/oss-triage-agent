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
	"github.com/harsh-m-patil/oss-triage-agent/internal/agent/opencode"
	"github.com/spf13/cobra"
)

var (
	agentModel                      string
	agentPrompt                     string
	agentVariant                    string
	agentName                       string
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
	Long: `Run an OpenCode agent and emit normalized Agent events as JSON lines on stdout.

Useful for debugging stream parsing and agent configuration before wiring a
full workflow. Prompt via --prompt or stdin.`,
	Example: `  oss-triage-agent agent run --prompt "Summarize this repo"
  echo "What changed?" | oss-triage-agent agent run`,
	RunE: runAgent,
}

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentRunCmd)

	agentRunCmd.Flags().StringVar(&agentModel, "model", "opencode/big-pickle", "Model passed to the agent provider")
	agentRunCmd.Flags().StringVarP(&agentPrompt, "prompt", "p", "", "Prompt (reads stdin when empty)")
	agentRunCmd.Flags().StringVar(&agentVariant, "variant", "", "OpenCode --variant flag")
	agentRunCmd.Flags().StringVar(&agentName, "agent", "", "OpenCode --agent flag")
	agentRunCmd.Flags().BoolVar(&agentDangerouslySkipPermissions, "dangerously-skip-permissions", false, "OpenCode --dangerously-skip-permissions flag")
}

func runAgent(cmd *cobra.Command, args []string) error {
	if _, err := exec.LookPath("opencode"); err != nil {
		return fmt.Errorf("opencode binary not found on PATH: %w", err)
	}

	prompt, err := resolveAgentPrompt(agentPrompt)
	if err != nil {
		return err
	}

	provider := opencode.NewProvider(agentModel, opencode.Options{
		Variant:                    agentVariant,
		Agent:                      agentName,
		DangerouslySkipPermissions: agentDangerouslySkipPermissions,
		Env:                        opencodeEnvFromOS(),
	})

	return streamAgentEvents(cmd.Context(), provider, prompt, os.Stdout, os.Stderr)
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

func opencodeEnvFromOS() map[string]string {
	if v := os.Getenv("OPENCODE_API_KEY"); v != "" {
		return map[string]string{"OPENCODE_API_KEY": v}
	}
	return nil
}

func streamAgentEvents(ctx context.Context, provider agent.AgentProvider, prompt string, out, errOut io.Writer) error {
	argv := provider.BuildCommand(prompt)
	if len(argv) == 0 {
		return fmt.Errorf("provider %q returned empty command", provider.Name())
	}

	command := argv[0]
	args := argv[1:]

	execCmd := exec.CommandContext(ctx, command, args...)
	execCmd.Env = os.Environ()
	for k, v := range provider.Env() {
		execCmd.Env = append(execCmd.Env, k+"="+v)
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
