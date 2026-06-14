package tui

import tea "github.com/charmbracelet/bubbletea"

// handleKey routes key input to the active screen.
func (a *App) handleKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.String() == "ctrl+c" {
		return a, tea.Quit
	}
	switch a.screen {
	case screenSplash:
		a.screen = a.postSplash // any key skips the splash
		return a, nil
	case screenSetup:
		return a.setupKey(m)
	case screenDest:
		return a.destKey(m)
	case screenBrowse:
		return a.browseKey(m)
	case screenConfirm:
		return a.confirmKey(m)
	case screenLoadoutPicker:
		return a.loadoutPickerKey(m)
	case screenLoadoutName:
		return a.loadoutNameKey(m)
	case screenDone:
		return a, tea.Quit
	case screenLoading, screenSyncing:
		// ignore keys while working (ctrl+c handled above)
		return a, nil
	}
	return a, nil
}
