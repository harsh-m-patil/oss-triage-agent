package orchestrator_test

import (
	"bytes"
	"os/exec"
	"strings"
)

func listDeps(importPath string) []string {
	cmd := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", importPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}
	var deps []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line != "" {
			deps = append(deps, line)
		}
	}
	return deps
}
