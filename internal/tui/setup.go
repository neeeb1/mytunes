package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neeeb1/mytunes/internal/config"
)

// setupStepDef describes one screen of the first-run wizard.
type setupStepDef struct {
	prompt      string
	placeholder string
	optional    bool
	completable bool               // local path, supports Tab completion
	get         func(*App) *string // where the entered value is stored
}

var setupSteps = []setupStepDef{
	{
		prompt:      "SSH remote (user@host):",
		placeholder: "you@192.168.1.10",
		get:         func(a *App) *string { return &a.setupRemote },
	},
	{
		prompt:      "Remote music path:",
		placeholder: "/srv/music",
		get:         func(a *App) *string { return &a.setupPath },
	},
	{
		prompt:      "Default destination (optional):",
		placeholder: "/run/media/you/USB/Music",
		optional:    true,
		completable: true,
		get:         func(a *App) *string { return &a.setupDest },
	},
}

// loadSetupStep points the input at the current step, pre-filled with any value
// already entered for it.
func (a *App) loadSetupStep() {
	st := setupSteps[a.setupStep]
	a.setupInput.Placeholder = st.placeholder
	a.setupInput.SetValue(*st.get(a))
	a.setupInput.CursorEnd()
	a.setupInput.Focus()
	a.setupErr = ""
	a.resetCompletion()
}

func (a *App) setupKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	st := setupSteps[a.setupStep]
	switch m.String() {
	case "tab":
		// Only the destination step is a local path worth completing.
		if st.completable {
			a.setupInput.SetValue(a.tabComplete(a.setupInput.Value()))
			a.setupInput.CursorEnd()
		}
		return a, nil
	case "esc":
		if a.setupStep == 0 {
			return a, tea.Quit
		}
		*st.get(a) = a.setupInput.Value()
		a.setupStep--
		a.loadSetupStep()
		return a, nil
	case "enter":
		val := strings.TrimSpace(a.setupInput.Value())
		if val == "" && !st.optional {
			a.setupErr = "this field is required"
			return a, nil
		}
		if a.setupStep == 0 && !strings.Contains(val, "@") {
			a.setupErr = "expected user@host"
			return a, nil
		}
		*st.get(a) = val
		if a.setupStep < len(setupSteps)-1 {
			a.setupStep++
			a.loadSetupStep()
			return a, nil
		}
		return a.finishSetup()
	default:
		a.resetCompletion()
		var cmd tea.Cmd
		a.setupInput, cmd = a.setupInput.Update(m)
		return a, cmd
	}
}

// finishSetup persists the collected config and advances to the destination
// picker, pre-filled with the chosen default destination.
func (a *App) finishSetup() (tea.Model, tea.Cmd) {
	a.cfg = a.cfg.Apply(config.Overrides{
		Remote: a.setupRemote,
		Path:   a.setupPath,
		Dest:   a.setupDest,
	})
	if err := a.cfg.Save(); err != nil {
		a.setupErr = "could not save config: " + err.Error()
		return a, nil
	}

	a.setupInput.Blur()
	a.destInput.SetValue(a.cfg.LastDest)
	a.destInput.CursorEnd()
	a.destInput.Focus()
	a.err = nil
	a.screen = screenDest
	return a, nil
}

func (a *App) viewSetup() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("myTunes — setup") + "\n\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Step %d of %d", a.setupStep+1, len(setupSteps))) + "\n\n")

	st := setupSteps[a.setupStep]
	b.WriteString(st.prompt + "\n")
	b.WriteString(a.setupInput.View() + "\n")
	b.WriteString(completionHint(a.pathCands, a.pathActive) + "\n")

	if a.setupErr != "" {
		b.WriteString(errStyle.Render(a.setupErr) + "\n")
	}
	b.WriteByte('\n')

	back := "esc quit"
	if a.setupStep > 0 {
		back = "esc back"
	}
	help := "enter next · " + back
	if st.completable {
		help = "tab complete · " + help
	}
	b.WriteString(helpStyle.Render(help))
	return b.String()
}
