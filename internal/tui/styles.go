package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/neeeb1/mytunes/internal/diff"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	artistStyle = lipgloss.NewStyle().Bold(true)

	syncedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	remoteOnlyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	localOnlyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	modifiedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))

	tagCopy   = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	tagUpdate = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	tagDelete = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

func stateStyle(s diff.State) lipgloss.Style {
	switch s {
	case diff.Synced:
		return syncedStyle
	case diff.RemoteOnly:
		return remoteOnlyStyle
	case diff.LocalOnly:
		return localOnlyStyle
	case diff.Modified:
		return modifiedStyle
	}
	return dimStyle
}

func actionTag(a diff.Action) string {
	switch a {
	case diff.Copy:
		return tagCopy.Render(a.Tag())
	case diff.Update:
		return tagUpdate.Render(a.Tag())
	case diff.Delete:
		return tagDelete.Render(a.Tag())
	}
	return ""
}
