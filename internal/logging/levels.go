package logging

import (
	"strings"

	charm "github.com/charmbracelet/log"
	"github.com/charmbracelet/lipgloss"
)

// Custom charm log levels for AFK workflow output. Values sit between the
// built-in info and warn levels so agent progress is visible at the default
// info threshold but can be filtered independently later.
const (
	LevelAgent     charm.Level = 1
	LevelTool      charm.Level = 2
	LevelUsage     charm.Level = 3
	LevelStderr    charm.Level = 5
	LevelHeartbeat charm.Level = 6
)

func applyBuildStyles(l *charm.Logger) {
	if l == nil {
		return
	}
	st := charm.DefaultStyles()
	st.Levels[LevelAgent] = levelStyle("AGENT", "86")
	st.Levels[LevelTool] = levelStyle("TOOL", "141")
	st.Levels[LevelUsage] = levelStyle("USAGE", "42")
	st.Levels[LevelStderr] = levelStyle("STDERR", "192")
	st.Levels[LevelHeartbeat] = levelStyle("WAIT", "244")
	l.SetStyles(st)
}

func levelStyle(label, color string) lipgloss.Style {
	return lipgloss.NewStyle().
		SetString(strings.ToUpper(label)).
		Bold(true).
		MaxWidth(len(label)).
		Foreground(lipgloss.Color(color))
}
