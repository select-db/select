import type { Icons } from '$lib/system/Icon/types';

/** Metadata shape for DB column nodes (subset used for icon choice). */
export type ColumnMetadata = { isPrimaryKey?: boolean };

export interface IconConfig {
	icon: Icons;
	spacer: boolean;
	size?: number;
}

/**
 * O(1) exact-match lookup for all simple type → icon mappings.
 * Column prefix types and the primary-key override are handled separately below.
 */
const EXACT_ICON_MAP = new Map<string, IconConfig>([
	['db_instance', { icon: 'db', spacer: false }],
	['schema', { icon: 'schema', spacer: false }],
	['table', { icon: 'table', spacer: false }],
	['view', { icon: 'table', spacer: false }],
	['index', { icon: 'index', spacer: true }],
	['trigger', { icon: 'bolt', spacer: false }],
	['column:integer', { icon: 'number', spacer: true }],
	['column:jsonb', { icon: 'json', spacer: true }],
	['column:json', { icon: 'json', spacer: true }],
	['column:boolean', { icon: 'checkbox', spacer: true }],
	['quick_action:query', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:schema', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:chat', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:terminal', { icon: 'chevron-right', spacer: false, size: 19 }],
	['quick_action:settings', { icon: 'chevron-right', spacer: false, size: 19 }],
	['settings', { icon: 'cog', spacer: false, size: 19 }],
	['chat', { icon: 'chat', spacer: false }],
	['terminal', { icon: 'terminal', spacer: false }],
	['diff', { icon: 'diff', spacer: false }],
	['type', { icon: 'code-bracket', spacer: false }],
	['function', { icon: 'function', spacer: false }],
	['db_setting', { icon: 'code-bracket', spacer: false }]
]);

/**
 * Returns icon and spacer for a type (and optional metadata). Used by ItemIcon and modal headers.
 */
export function getIconConfig(type?: string): IconConfig | undefined {
	if (!type) return { icon: 'text', spacer: false };
	if (type === 'file') return { icon: 'paragraph', spacer: false };

	// O(1) exact match covers the vast majority of types
	const exact = EXACT_ICON_MAP.get(type);
	if (exact) return exact;

	// Prefix fallbacks for remaining column variants
	if (type.startsWith('column:character') || type.startsWith('column:text'))
		return { icon: 'text', spacer: true };
	if (type.startsWith('column:timestamp') || type.startsWith('column:datetime'))
		return { icon: 'timestamp', spacer: true };
	if (type.startsWith('column:')) return { icon: 'code-bracket', spacer: true };
	if (type.includes('[]')) return { icon: 'brackets', spacer: false };

	return undefined;
}

/**
 * Returns only the icon. Use when you don't need the spacer (e.g. modal header, buttons).
 */
export function getIcon(type?: string): Icons | undefined {
	return getIconConfig(type)?.icon;
}

type FileIconDef = { icon: Icons; size: number; color: string };

const SPECIAL_FILENAMES: Record<string, FileIconDef> = {
	'[edits].sql': { icon: 'db-upload', size: 16, color: 'var(--gray-800)' },
	'.gitignore': { icon: 'branch', size: 20, color: 'var(--blue)' }
};

const EXTENSION_ICONS: Record<string, FileIconDef> = {
	'.theme': { icon: 'css', size: 20, color: 'var(--orange)' },
	'.config': { icon: 'cog', size: 19, color: 'var(--gray-800)' },
	'.lint': { icon: 'eslint', size: 20, color: 'var(--purple)' },
	'.css': { icon: 'css', size: 20, color: 'var(--blue)' },
	'.html': { icon: 'html', size: 20, color: 'var(--red)' },
	'.js': { icon: 'js', size: 20, color: 'var(--yellow)' },
	'.sql': { icon: 'sql', size: 20, color: 'var(--red)' },
	'.env': { icon: 'code-bracket', size: 19, color: 'var(--gray-800)' },
	'.ts': { icon: 'ts', size: 20, color: 'var(--blue)' },
	'.csv': { icon: 'csv', size: 20, color: 'var(--gray-800)' },
	'.json': { icon: 'json', size: 19, color: 'var(--gray-800)' }
};

function getFileExtension(fileName: string): string {
	const lastDot = fileName.lastIndexOf('.');
	return lastDot !== -1 ? fileName.slice(lastDot) : '';
}

function getFileIconConfig(fileName: string): FileIconDef {
	const specialMatch = SPECIAL_FILENAMES[fileName];
	if (specialMatch) return specialMatch;

	const ext = getFileExtension(fileName);
	const extMatch = EXTENSION_ICONS[ext];
	if (extMatch) return extMatch;

	return { icon: 'paragraph', size: 18, color: 'var(--gray-800)' };
}

/** Icon for file nodes (e.g. modal). For size/color, use getItemIconSlot file slot. */
export function getFileIcon(fileName: string): Icons {
	return getFileIconConfig(fileName).icon;
}

/** Content-only slot (no expand icon). */
export type ItemIconContentSlot =
	| { kind: 'file'; icon: Icons; size: number; color: string }
	| { kind: 'database-indicator'; id: string }
	| { kind: 'icon'; icon: Icons; spacer: boolean; size?: number };

/** Container types that show only a folder icon (no content icon) in the tree. */
const FOLDER_ONLY_TYPES = new Set([
	'folder',
	'tables',
	'views',
	'triggers',
	'indexes',
	'columns',
	'types',
	'functions',
	'db_settings'
]);

/** Icon for expand/collapse: folder (collapsed) or folder-open (expanded). */
export const EXPAND_ICON_COLLAPSED: Icons = 'folder';
export const EXPAND_ICON_EXPANDED: Icons = 'folder-open';

/** Full slot: expandable (folder icon + content), folder-only, or content-only. */
export type ItemIconSlot =
	| { kind: 'expandable'; expandIcon: Icons; content: ItemIconContentSlot }
	| { kind: 'folder-only'; expandIcon: Icons }
	| ItemIconContentSlot;

export interface ItemIconSlotInput {
	type: string;
	name?: string;
	id?: string;
	metadata?: Record<string, unknown>;
}

export interface ItemIconSlotOptions {
	noDepth: boolean;
	expanded: boolean;
	expandable: boolean;
	muted: boolean;
}

/** Compute the content slot (icon / database-indicator / file) without expand icon logic. */
function getItemIconContentSlot(
	item: ItemIconSlotInput,
	noDepth: boolean
): ItemIconContentSlot | undefined {
	if (item.type === 'file' && item.name != null) {
		const fileConfig = getFileIconConfig(item.name);
		return { kind: 'file', icon: fileConfig.icon, size: fileConfig.size, color: fileConfig.color };
	}

	if (item.type === 'db_instance' && item.id != null) {
		return { kind: 'database-indicator', id: item.id };
	}

	const config = getIconConfig(item.type);
	if (!config) return undefined;

	return {
		kind: 'icon',
		icon: config.icon,
		spacer: noDepth ? false : config.spacer,
		size: config.size
	};
}

/**
 * Determines what ItemIcon should render for the given item and expansion state.
 * When expandable/expanded and !noDepth, returns expandable (folder icon + content) or folder-only for container types.
 * Returns undefined if no icon should be rendered.
 */
export function getItemIconSlot(
	item: ItemIconSlotInput,
	options: ItemIconSlotOptions
): ItemIconSlot | undefined {
	const { noDepth, expanded, expandable } = options;

	if (!noDepth && expanded) {
		if (FOLDER_ONLY_TYPES.has(item.type ?? '')) {
			return { kind: 'folder-only', expandIcon: EXPAND_ICON_EXPANDED };
		}

		const content = getItemIconContentSlot(item, noDepth);
		if (!content) return undefined;

		return { kind: 'expandable', expandIcon: EXPAND_ICON_EXPANDED, content };
	}
	if (!noDepth && expandable) {
		if (FOLDER_ONLY_TYPES.has(item.type ?? '')) {
			return { kind: 'folder-only', expandIcon: EXPAND_ICON_COLLAPSED };
		}

		const content = getItemIconContentSlot(item, noDepth);
		if (!content) return undefined;

		return { kind: 'expandable', expandIcon: EXPAND_ICON_COLLAPSED, content };
	}

	return getItemIconContentSlot(item, noDepth);
}
