package orchestrator_test

import (
	"strings"
	"testing"
)

func TestOrchestratorPackage_doesNotDependOnConcreteProviders(t *testing.T) {
	t.Parallel()

	// Guardrail: orchestration must stay behind interfaces, not Docker/OpenCode/etc.
	forbidden := []string{"docker", "opencode"}
	for _, dep := range depsOf("github.com/harsh-m-patil/oss-triage-agent/internal/orchestrator") {
		low := strings.ToLower(dep)
		for _, needle := range forbidden {
			if strings.Contains(low, needle) {
				t.Fatalf("orchestrator depends on %q via %q", needle, dep)
			}
		}
	}
}

func depsOf(importPath string) []string {
	seen := map[string]bool{}
	var walk func(string)
	walk = func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		for _, dep := range listDeps(path) {
			walk(dep)
		}
	}
	walk(importPath)
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}
