import type { Component } from 'svelte';
import { get } from 'svelte/store';
import {
	isLeftbarOpened,
	updateIsLeftbarOpened,
	toggleLeftPanelTab
} from '$lib/components/Leftbar/store';
import {
	isRightbarOpened,
	updateIsRightbarOpened,
	togglePanelTab
} from '$lib/components/Rightbar/rightbarStore';
import { modalStore } from '$lib/system/Modal/ModalStore';
import SearchModalContent from '$lib/components/Leftbar/Search/SearchModalContent.svelte';

import { registerCommand } from './commandRegistry';
import { zoomIn, zoomOut, resetZoom } from '$lib/wails/zoom';
import {
	addTerminalTab,
	addTempFileTab,
	getActiveTab,
	getActiveGroup,
	removeTab,
	navigateToPreviousTab,
	navigateToNextTab
} from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';

export function registerWorkbenchCommands(): void {
	registerCommand('workbench.toggleLeftPanel', () => {
		updateIsLeftbarOpened(!get(isLeftbarOpened));
	});

	registerCommand('workbench.toggleRightPanel', () => {
		updateIsRightbarOpened(!get(isRightbarOpened));
	});

	registerCommand('workbench.toggleFiles', () => toggleLeftPanelTab('files'));
	registerCommand('workbench.toggleGit', () => toggleLeftPanelTab('github-branch'));
	registerCommand('workbench.toggleSearch', () => togglePanelTab('search'));
	registerCommand('workbench.toggleHistory', () => togglePanelTab('history'));

	registerCommand('workbench.openSearch', async () => {
		modalStore.set({
			content: () => SearchModalContent as unknown as Component,
			width: 650
		});
	});

	registerCommand('workbench.zoomIn', zoomIn);
	registerCommand('workbench.zoomOut', zoomOut);
	registerCommand('workbench.zoomReset', resetZoom);
	registerCommand('workbench.openTerminal', addTerminalTab);
	registerCommand('workbench.newSqlFile', () => {
		const folderId = get(workspaceGraphStore)?.folders?.[0]?.id ?? '';
		const activeTab = getActiveTab();
		const dbInstanceId =
			activeTab?.database?.node.id ?? activeTab?.file?.node.databases?.[0]?.id ?? undefined;
		addTempFileTab({ content: '', name: '[temp].sql', folderId, dbInstanceId }, false);
	});

	registerCommand('workbench.closeActiveTab', () => {
		const tab = getActiveTab();
		if (tab) removeTab(tab.id);
	});

	registerCommand('workbench.previousTabInGroup', () => {
		const group = getActiveGroup();
		if (group) navigateToPreviousTab(group.id);
	});

	registerCommand('workbench.nextTabInGroup', () => {
		const group = getActiveGroup();
		if (group) navigateToNextTab(group.id);
	});
}
