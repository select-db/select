import { writable } from 'svelte/store';

export type LeftPanelTab = 'files' | 'github-branch';

const LEFT_PANEL_TAB_KEY = 'leftPanelTab';

let internalIsLeftbarOpened = JSON.parse(localStorage.getItem('isLeftbarOpened') ?? 'true');
let internalLeftPanelTab: LeftPanelTab =
	(localStorage.getItem(LEFT_PANEL_TAB_KEY) as LeftPanelTab) ?? 'files';

export const isLeftbarOpened = writable(internalIsLeftbarOpened);
export const leftPanelTab = writable<LeftPanelTab>(internalLeftPanelTab);

export const updateIsLeftbarOpened = (v: boolean) => {
	if (v === internalIsLeftbarOpened) return;
	internalIsLeftbarOpened = v;
	localStorage.setItem('isLeftbarOpened', JSON.stringify(v));
	isLeftbarOpened.update(() => v);
};

export const updateLeftPanelTab = (tab: LeftPanelTab) => {
	if (tab === internalLeftPanelTab) return;
	internalLeftPanelTab = tab;
	localStorage.setItem(LEFT_PANEL_TAB_KEY, tab);
	leftPanelTab.update(() => tab);
};

/** Open left panel with tab, close if already on that tab, or switch to tab. */
export function toggleLeftPanelTab(tab: LeftPanelTab): void {
	if (!internalIsLeftbarOpened) {
		updateLeftPanelTab(tab);
		updateIsLeftbarOpened(true);
	} else if (internalLeftPanelTab === tab) {
		updateIsLeftbarOpened(false);
	} else {
		updateLeftPanelTab(tab);
	}
}
