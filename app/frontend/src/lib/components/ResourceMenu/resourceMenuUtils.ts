import type * as graph from '$lib/wails/graph';
import { getTabLabel, type Tab, type TabGroup, type SplitContainer } from '$lib/components/Layout/layoutStore';
import type { ResourceType, ResourceMenuOption, ResourceSearchScope } from './types';
import type { RecentItem } from '$lib/stores/recentItemsStore';
import { resourceOptionInSearchScope } from './resourceMenuScope';

/** Cap list size for menu render + sort cost. */
export const MAX_RESOURCE_MENU_OPTIONS = 50;

/** Tables/views first, then columns, then indexes/functions/triggers/types. */
function dbItemTypePriority(opt: ResourceMenuOption): number {
	if (opt.type !== 'db_item') return 0;
	const t = (opt.node as graph.DBInstanceItemNode).type;
	if (t === 'table' || t === 'view') return 0;
	if (t.startsWith('column:')) return 1;
	return 2;
}

// TODO: refacto with back (or front) btree index ?
export function flattenWorkspaceGraph(
	workspace: graph.WorkspaceNode | undefined,
	types: ResourceType[]
): ResourceMenuOption[] {
	if (!workspace) return [];

	const options: ResourceMenuOption[] = [];

	const pushFile = (file: graph.FileNode) => {
		options.push({
			id: file.id,
			label: file.name,
			type: 'file',
			uri: file.uri,
			node: file,
			folderId: file.folder_id
		});
	};

	const processDbInstance = (db: graph.DBInstanceNode) => {
		if (types.includes('db_instance')) {
			options.push({
				id: db.id,
				label: db.name,
				type: 'db_instance',
				uri: db.uri,
				node: db
			});
		}

		if (types.includes('db_item') && db.children) {
			collectDbItems(db.children, options);
		}

		// SQL files (and nested folders) can live inside a database, not just in
		// regular workspace folders — index them too.
		if (types.includes('file')) {
			for (const file of db.files) pushFile(file);
		}
		for (const subFolder of db.folders) processFolder(subFolder);
	};

	const processFolder = (folder: graph.FolderNode) => {
		if (types.includes('file')) {
			for (const file of folder.files) pushFile(file);
		}

		for (const db of folder.db_instances) processDbInstance(db);

		for (const subFolder of folder.folders) processFolder(subFolder);
	};

	for (const folder of workspace.folders) {
		processFolder(folder);
	}

	for (const db of workspace.db_instances) processDbInstance(db);

	return options;
}

const EXCLUDED_DB_ITEM_TYPES = [
	'columns',
	'column',
	'index',
	'index:column',
	'tables',
	'views',
	'indexes',
	'triggers'
];

function collectDbItems(items: graph.DBInstanceItemNode[], options: ResourceMenuOption[]): void {
	for (const item of items) {
		const isExcluded = EXCLUDED_DB_ITEM_TYPES.some(
			(t) => item.type === t || item.type.startsWith('column:')
		);

		if (!isExcluded) {
			options.push({
				id: item.id,
				label: item.name,
				type: 'db_item',
				uri: item.uri,
				node: item
			});
		}

		if (item.children?.length) {
			collectDbItems(item.children, options);
		}
	}
}

function collectTabGroups(node: TabGroup | SplitContainer): TabGroup[] {
	if ('type' in node && node.type === 'split') {
		return node.children.flatMap(collectTabGroups);
	}
	return [node as TabGroup];
}

function tabNode(id: string, name: string, type: string, uri: string): graph.FileNode {
	return {
		id,
		name,
		type,
		path: '',
		uri,
		folder_id: '',
		badges: [],
		convertValues: () => ({})
	} as unknown as graph.FileNode;
}

function tabType(tab: Tab): ResourceType | null {
	if (tab.settings) return 'settings';
	if (tab.schema) return 'schema';
	if (tab.chat) return 'chat';
	if (tab.terminal) return 'terminal';
	if (tab.database) return 'db_instance';
	if (tab.diff) return 'diff';
	return null;
}

export function getOpenTabOptions(
	layoutRoot: TabGroup | SplitContainer,
	types: ResourceType[]
): ResourceMenuOption[] {
	const options: ResourceMenuOption[] = [];
	const groups = collectTabGroups(layoutRoot);

	for (const group of groups) {
		for (const tab of group.tabs) {
			// Temp files
			if (
				tab.file?.isTemp &&
				tab.file.node &&
				(types.includes('temp_file') || types.includes('file'))
			) {
				options.push({
					id: tab.id,
					label: tab.file.node.name,
					type: 'temp_file',
					uri: tab.uri,
					node: tab.file.node,
					folderId: tab.file.node.folder_id
				});
				continue;
			}

			// Non-file tabs (settings, schema, chat, terminal, database, diff)
			if (tab.file) continue;

			const type = tabType(tab);
			if (!type || !types.includes(type)) continue;

			const label = getTabLabel(tab);
			options.push({
				id: tab.id,
				label,
				type,
				uri: tab.uri,
				node: tab.database?.node ?? tabNode(tab.id, label, type, tab.uri)
			});
		}
	}

	return options;
}

/**
 * Merges, dedupes, filters, and sorts options.
 * Dedupes by ID (always unique), with URI as fallback for same-resource detection.
 */
export function filterOptions(
	graphOptions: ResourceMenuOption[],
	tabOptions: ResourceMenuOption[],
	recentItems: RecentItem[],
	types: ResourceType[],
	excludeIds: string[],
	searchQuery: string,
	searchScope?: ResourceSearchScope
): ResourceMenuOption[] {
	const excludeSet = new Set(excludeIds);
	const query = searchQuery.toLowerCase().trim();

	// Dedupe by ID, and also track non-empty URIs to avoid duplicates from different sources
	const seenIds = new Set<string>();
	const seenUris = new Set<string>();
	const merged: ResourceMenuOption[] = [];

	for (const opt of graphOptions) {
		if (seenIds.has(opt.id)) continue;
		if (opt.uri && seenUris.has(opt.uri)) continue;
		if (excludeSet.has(opt.id)) continue;
		if (searchScope && !resourceOptionInSearchScope(opt, searchScope)) continue;
		if (query && !opt.label.toLowerCase().includes(query)) continue;

		seenIds.add(opt.id);
		if (opt.uri) seenUris.add(opt.uri);
		merged.push(opt);
	}

	for (const opt of tabOptions) {
		if (seenIds.has(opt.id)) continue;
		if (opt.uri && seenUris.has(opt.uri)) continue;
		if (excludeSet.has(opt.id)) continue;
		if (query && !opt.label.toLowerCase().includes(query)) continue;

		seenIds.add(opt.id);
		if (opt.uri) seenUris.add(opt.uri);
		merged.push(opt);
	}

	// Build recent URIs for sorting
	const recentUris = new Set(
		recentItems.filter((item) => types.includes(item.type)).map((item) => item.uri)
	);

	// Sort
	return merged
		.sort((a, b) => {
			// 1. Type priority: tables/views always before indexes/functions/triggers
			const aPri = dbItemTypePriority(a);
			const bPri = dbItemTypePriority(b);
			if (aPri !== bPri) return aPri - bPri;

			const aLabel = a.label.toLowerCase();
			const bLabel = b.label.toLowerCase();

			// 2. Query match quality (within same type tier)
			if (query) {
				const aExact = aLabel === query;
				const bExact = bLabel === query;
				if (aExact !== bExact) return aExact ? -1 : 1;

				const aStarts = aLabel.startsWith(query);
				const bStarts = bLabel.startsWith(query);
				if (aStarts !== bStarts) return aStarts ? -1 : 1;

				// Word-boundary match: query starts a segment after _ - . or space
				const aWord = ['_', '-', '.', ' '].some((sep) => aLabel.includes(sep + query));
				const bWord = ['_', '-', '.', ' '].some((sep) => bLabel.includes(sep + query));
				if (aWord !== bWord) return aWord ? -1 : 1;
			}

			// 3. Recent items first
			const aRecent = recentUris.has(a.uri);
			const bRecent = recentUris.has(b.uri);
			if (aRecent !== bRecent) return aRecent ? -1 : 1;

			if (aRecent && bRecent) {
				const aIdx = recentItems.findIndex((r) => r.uri === a.uri);
				const bIdx = recentItems.findIndex((r) => r.uri === b.uri);
				return aIdx - bIdx;
			}

			// 4. Alphabetical
			return a.label.localeCompare(b.label);
		})
		.slice(0, MAX_RESOURCE_MENU_OPTIONS);
}
