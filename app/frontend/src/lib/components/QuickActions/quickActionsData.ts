import { get } from 'svelte/store';
import {
	addTempFileTab,
	addSchemaTab,
	addChatTab,
	addTerminalTab,
	addSettingsTab
} from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import type { ResourceMenuOption } from '$lib/components/ResourceMenu/types';
import type { graph } from '$lib/wailsjs/go/models';

function createQuickActionNode(type: string, name: string): graph.FileNode {
	return {
		id: '',
		name,
		type,
		path: '',
		uri: '',
		folder_id: '',
		badges: [],
		convertValues: () => ({})
	} as unknown as graph.FileNode;
}

export const quickActions: ResourceMenuOption[] = [
	{
		id: 'quick-action-new-query',
		label: 'New query',
		type: 'quick_action',
		uri: '',
		node: createQuickActionNode('quick_action:query', 'New query')
	},
	{
		id: 'quick-action-open-schema',
		label: 'New Schema',
		type: 'quick_action',
		uri: '',
		node: createQuickActionNode('quick_action:schema', 'Open schema')
	},
	{
		id: 'quick-action-open-chat',
		label: 'New Chat',
		type: 'quick_action',
		uri: '',
		node: createQuickActionNode('quick_action:chat', 'Open chat')
	},
	{
		id: 'quick-action-open-terminal',
		label: 'New Terminal',
		type: 'quick_action',
		uri: '',
		node: createQuickActionNode('quick_action:terminal', 'Open terminal')
	},
	{
		id: 'quick-action-open-settings',
		label: 'Open Settings',
		type: 'quick_action',
		uri: '',
		node: createQuickActionNode('quick_action:settings', 'Open settings')
	}
];

export function executeQuickAction(option: ResourceMenuOption): void {
	if (option.type !== 'quick_action') return;
	const workspace = get(workspaceGraphStore);
	const folderId = workspace?.folders?.[0]?.id ?? '';
	if (option.id === 'quick-action-new-query') {
		addTempFileTab({ content: '', name: '[temp].sql', folderId }, false);
	} else if (option.id === 'quick-action-open-schema') {
		addSchemaTab();
	} else if (option.id === 'quick-action-open-chat') {
		addChatTab();
	} else if (option.id === 'quick-action-open-terminal') {
		addTerminalTab();
	} else if (option.id === 'quick-action-open-settings') {
		addSettingsTab();
	}
}
