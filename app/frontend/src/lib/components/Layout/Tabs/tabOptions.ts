import type { ContextMenuOption } from '$lib/system/ContextMenu/types';

import { fileSystemOptions } from '$lib/components/views/FileSystem/Files/options/fileOptions';

import { removeTab, type Tab, getAllGroups } from '../layoutStore';

export const tabOptions = [
	{
		label: 'Close',
		action: (onClose, { id }: Tab) => {
			removeTab(id);
			onClose();
		}
	},
	{
		label: 'Close others',
		action: (onClose, { id }: Tab) => {
			// Find which group this tab belongs to and close all other tabs in that group
			const groups = getAllGroups();
			const group = groups.find((g) => g.tabs.some((t) => t.id === id));
			if (group) {
				group.tabs.forEach((tab) => {
					if (tab.id === id) return;
					removeTab(tab.id);
				});
			}
			onClose();
		}
	},
	{
		label: 'Close all',
		action: (onClose, { id }: Tab) => {
			// Close all tabs in the group this tab belongs to
			const groups = getAllGroups();
			const group = groups.find((g) => g.tabs.some((t) => t.id === id));
			if (group) {
				group.tabs.forEach((tab) => {
					removeTab(tab.id);
				});
			}
			onClose();
		}
	},
	...fileSystemOptions
] satisfies ContextMenuOption[];
