package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// logoArt is the myTunes wordmark shown on the splash screen.
const logoArt = `
::::    ::::  :::   ::: ::::::::::: :::    ::: ::::    ::: :::::::::: ::::::::
+:+:+: :+:+:+ :+:   :+:     :+:     :+:    :+: :+:+:   :+: :+:       :+:    :+:
+:+ +:+:+ +:+  +:+ +:+      +:+     +:+    +:+ :+:+:+  +:+ +:+       +:+
+#+  +:+  +#+   +#++:       +#+     +#+    +:+ +#+ +:+ +#+ +#++:++#  +#++:++#++
+#+       +#+    +#+        +#+     +#+    +#+ +#+  +#+#+# +#+              +#+
#+#       #+#    #+#        #+#     #+#    #+# #+#   #+#+# #+#       #+#    #+#
###       ###    ###        ###      ########  ###    #### ########## ########  `

// logoMark is a small placeholder disc mark above the wordmark. It is kept
// separate and trivial on purpose — swap it for your own ASCII art.
const logoMark = `      _____
   .-'     '-.
  /   .---.   \
 |   ( ( ) )   |
  \   '---'   /
   '-._____.-'`

// logoPalette is the aqua→blue gradient applied line-by-line to logoArt,
// top (bright cyan) to bottom (blue).
var logoPalette = []lipgloss.Color{"51", "45", "39", "33", "27", "26", "25"}

var (
	brandStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
	markStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
	taglineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("45"))
)

// splashTick schedules the auto-dismiss of the splash screen.
func splashTick() tea.Cmd {
	return tea.Tick(1800*time.Millisecond, func(time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

// renderLogo colors the wordmark with the aqua gradient. When the terminal is
// narrower than the art, it falls back to the compact "myTunes" wordmark.
func renderLogo(width int) string {
	lines := strings.Split(strings.Trim(logoArt, "\n"), "\n")

	logoWidth := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > logoWidth {
			logoWidth = w
		}
	}
	if width > 0 && width < logoWidth {
		return brandStyle.Render("myTunes")
	}

	var b strings.Builder
	for i, l := range lines {
		color := logoPalette[i%len(logoPalette)]
		b.WriteString(lipgloss.NewStyle().Foreground(color).Render(l))
		if i < len(lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// centerBlock horizontally centers each line of s within width w.
func centerBlock(s string, w int) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = lipgloss.PlaceHorizontal(w, lipgloss.Center, l)
	}
	return strings.Join(lines, "\n")
}

func (a *App) viewSplash() string {
	logo := renderLogo(a.width)
	w := lipgloss.Width(logo)

	parts := []string{
		centerBlock(markStyle.Render(logoMark), w),
		"",
		logo,
		"",
		centerBlock(taglineStyle.Render("remote music, synced"), w),
		centerBlock(helpStyle.Render("press any key"), w),
	}
	content := strings.Join(parts, "\n")

	if a.width > 0 && a.height > 0 {
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, content)
	}
	return content
}
