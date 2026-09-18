import { writable } from 'svelte/store';

export type RightPanelTab = 'search' | 'history';

const RIGHT_PANEL_TAB_KEY = 'rightPanelTab';

/** Shared by the bar (reads it) and its resizer (writes it) — keep them in sync. */
export const RIGHTBAR_WIDTH_KEY = 'rightbarWidth';

export const RIGHTBAR_MIN_WIDTH = 0;
export const RIGHTBAR_MAX_WIDTH = 420;
export const RIGHTBAR_DEFAULT_WIDTH = 300;

let internalIsRightbarOpened = JSON.parse(localStorage.getItem('isRightbarOpened') ?? 'false');
let internalRightPanelTab: RightPanelTab =
	(localStorage.getItem(RIGHT_PANEL_TAB_KEY) as RightPanelTab) ?? 'search';

export const isRightbarOpened = writable<boolean>(internalIsRightbarOpened);
export const rightPanelTab = writable<RightPanelTab>(internalRightPanelTab);

export const updateIsRightbarOpened = (v: boolean) => {
	if (v === internalIsRightbarOpened) return;
	internalIsRightbarOpened = v;
	localStorage.setItem('isRightbarOpened', JSON.stringify(v));
	isRightbarOpened.update(() => v);
};

export const updateRightPanelTab = (tab: RightPanelTab) => {
	if (tab === internalRightPanelTab) return;
	internalRightPanelTab = tab;
	localStorage.setItem(RIGHT_PANEL_TAB_KEY, tab);
	rightPanelTab.update(() => tab);
};

/**
 * Toggles a right-panel tab: opens the panel on that tab if closed, switches to
 * it if open on another tab, or closes the panel if already showing that tab.
 */
export function togglePanelTab(tab: RightPanelTab): void {
	if (!internalIsRightbarOpened) {
		updateRightPanelTab(tab);
		updateIsRightbarOpened(true);
	} else if (internalRightPanelTab === tab) {
		updateIsRightbarOpened(false);
	} else {
		updateRightPanelTab(tab);
	}
}
