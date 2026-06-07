package agent

// Launch is the argv and optional stdin for an agent subprocess.
// When Stdin is empty, the prompt is passed as the final argv element.
type Launch struct {
	Argv  []string
	Stdin string
}
