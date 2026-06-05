import { get } from 'svelte/store';
import type { Tab, ChatContextDatabase, ChatContextFile } from '$lib/components/Layout/layoutStore';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import type { CurrentFile } from '$lib/components/views/Chat/utils/currentFile';
import { currentFileStore } from '$lib/components/views/Chat/utils/currentFile';

/** Max chars for file/search/query results to avoid token overflow. */
export const TRUNCATE_AT = 8_000;

export function truncate(text: string, max = TRUNCATE_AT): string {
	if (text.length <= max) return text;
	return text.slice(0, max) + '\n\n...[truncated]';
}

export function toToolError(err: unknown): string {
	if (err instanceof Error) return err.message;
	return String(err);
}

/** Chat context: workspace, context DBs, files, and live current file from the active editor. */
export type ChatContext = {
	workspace?: { id: string; uri: string };
	databases?: ChatContextDatabase[];
	files?: ChatContextFile[];
	/** Live current file (uri, id, folderId, cursorPosition, textSelection) from currentFileStore. */
	currentFile?: CurrentFile | null;
};

/** Create a getter that derives context from layout and workspace stores. */
export function createDefaultGetContext(): () => ChatContext {
	return () => {
		const wsGraph = get(workspaceGraphStore);
		const workspaceId = wsGraph?.id ?? '';
		const workspaceUri = workspaceId ? `selectdb://workspaces/${workspaceId}` : '';
		const currentFile = get(currentFileStore);
		return {
			workspace: workspaceId ? { id: workspaceId, uri: workspaceUri } : undefined,
			databases: undefined,
			files: undefined,
			currentFile
		};
	};
}

/** Create a getter that returns context from a chat tab (databases, files, and live currentFile). */
export function getContextForChatTab(tab: Tab): ChatContext {
	const wsGraph = get(workspaceGraphStore);
	const workspaceId = wsGraph?.id ?? '';
	const workspaceUri = workspaceId ? `selectdb://workspaces/${workspaceId}` : '';
	const databases = tab.chat?.databases ?? [];
	const files = tab.chat?.files ?? [];
	const currentFile = get(currentFileStore);
	return {
		workspace: workspaceId ? { id: workspaceId, uri: workspaceUri } : undefined,
		databases,
		files,
		currentFile
	};
}
