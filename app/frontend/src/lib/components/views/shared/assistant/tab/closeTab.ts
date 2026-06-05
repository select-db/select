import type { Tab } from '$lib/components/Layout/layoutStore';
import * as TerminalBackend from '$lib/wailsjs/go/terminal/Terminal';

export function closeTab(tab: Tab) {
	if (tab.terminal) {
		TerminalBackend.Destroy(tab.terminal.sessionId).catch(() => {});
	}
}
