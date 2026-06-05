import { addDiffTab, getAllGroups } from '$lib/components/Layout/layoutStore';
import { getFileLanguage } from '$lib/components/views/File/Editor/utils/getFileLanguage';

export type OpenAgentDiffParams = {
	uri: string;
	originalContent: string;
	newContent: string;
	isTemp: boolean;
	language?: string;
	toolCallId?: string;
	/** Session ID of the originating chat, used to locate the chat group and pick the target group. */
	sessionId?: string;
};

function titleFromUri(uri: string): string {
	const segment = uri.split('/').filter(Boolean).pop();
	if (!segment) return 'New file';
	try {
		return decodeURIComponent(segment);
	} catch {
		return segment;
	}
}

/**
 * Returns the ID of the best group to open the diff in:
 * - any existing group that doesn't contain the originating chat tab
 * - null if no such group exists (caller should split instead)
 */
function findTargetGroupId(sessionId?: string): string | null {
	const groups = getAllGroups();
	if (groups.length <= 1) return null;

	const chatGroupId = sessionId
		? groups.find((g) => g.tabs.some((t) => t.chat?.sessionId === sessionId))?.id
		: null;

	const target = groups.find((g) => g.id !== chatGroupId);
	return target?.id ?? null;
}

export function openAgentDiffTab(params: OpenAgentDiffParams): void {
	const language = params.language ?? getFileLanguage(titleFromUri(params.uri));
	const targetGroupId = findTargetGroupId(params.sessionId);

	addDiffTab({
		panels: [
			{ content: params.originalContent, meta: { label: 'Original' } },
			{ content: params.newContent, meta: { label: 'Modified' } }
		],
		language,
		readOnly: false,
		viewMode: 'side-by-side',
		context: 'agent',
		meta: {
			uri: params.uri,
			isTemp: params.isTemp,
			...(params.toolCallId && { toolCallId: params.toolCallId })
		},
		title: titleFromUri(params.uri),
		...(targetGroupId ? { targetGroupId } : { split: true })
	});
}
