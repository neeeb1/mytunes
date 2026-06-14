package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) destKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc":
		return a, tea.Quit
	case "enter":
		return a, a.validateCmd(a.destInput.Value())
	case "tab":
		a.destInput.SetValue(a.tabComplete(a.destInput.Value()))
		a.destInput.CursorEnd()
		return a, nil
	default:
		a.resetCompletion()
		var cmd tea.Cmd
		a.destInput, cmd = a.destInput.Update(m)
		return a, cmd
	}
}

func (a *App) viewDest() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("mytunes — music sync") + "\n\n")
	b.WriteString("Pull " + dimStyle.Render(a.cfg.Remote()+":"+a.cfg.RemotePath) + " to a destination.\n\n")
	b.WriteString("Destination music directory:\n")
	b.WriteString(a.destInput.View() + "\n")
	b.WriteString(completionHint(a.pathCands, a.pathActive) + "\n")
	if a.err != nil {
		b.WriteString(errStyle.Render(a.err.Error()) + "\n\n")
	}
	b.WriteString(helpStyle.Render("tab complete · enter confirm (creates dir if missing) · esc quit"))
	return b.String()
}

func (a *App) viewLoading() string {
	msg := a.loadMsg
	if msg == "" {
		msg = "Working…"
	}
	return "\n  " + a.spinner.View() + " " + msg + "\n"
}
