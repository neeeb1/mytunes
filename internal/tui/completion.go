package tui

// tabComplete advances Tab completion for a local directory path and returns the
// new input value, updating the candidate/cycle state used for rendering:
//
//   - a unique match is filled in and descended into (trailing slash);
//   - an ambiguous match first extends to the longest common prefix and lists
//     the candidates;
//   - when there is no common-prefix progress, repeated Tab cycles through the
//     candidates, inserting each in turn (menu-style).
func (a *App) tabComplete(cur string) string {
	// Continue an active cycle while the value is still the candidate we last
	// inserted.
	if len(a.tabCycle) > 0 && a.tabIdx < len(a.tabCycle) && cur == a.tabCycle[a.tabIdx] {
		a.tabIdx = (a.tabIdx + 1) % len(a.tabCycle)
		a.pathActive = a.tabIdx
		return a.tabCycle[a.tabIdx]
	}
	a.resetCompletion()

	dir, base := splitPath(cur)
	matches := dirMatches(dir, base)
	switch len(matches) {
	case 0:
		return cur
	case 1:
		return dir + matches[0] + "/"
	}

	a.pathCands = matches

	lcp := matches[0]
	for _, m := range matches[1:] {
		lcp = commonPrefix(lcp, m)
	}
	if len(lcp) > len(base) {
		return dir + lcp // extend to the common prefix; just list, no cycle yet
	}

	// No new characters to add — cycle through the candidates instead.
	a.tabCycle = make([]string, len(matches))
	for i, m := range matches {
		a.tabCycle[i] = dir + m + "/"
	}
	a.tabIdx = 0
	a.pathActive = 0
	return a.tabCycle[0]
}

// resetCompletion clears Tab-completion state; call it on any non-Tab edit.
func (a *App) resetCompletion() {
	a.pathCands = nil
	a.pathActive = -1
	a.tabCycle = nil
	a.tabIdx = 0
}
