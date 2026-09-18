import type { Tab } from '$lib/components/Layout/layoutStore';
import * as TerminalBackend from '$lib/bindings/selectDb/internal/terminal/terminal';

export function closeTab(tab: Tab) {
	if (tab.terminal) {
		TerminalBackend.Destroy(tab.terminal.sessionId).catch(() => {});
	}
}
