import { toolDefinition } from '$lib/components/views/Chat/core/chat/tool-definition';
import { z } from 'zod';
import { getAllGroups, removeTab, updateTab } from '$lib/components/Layout/layoutStore';
import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';
import { tryCatch } from '$lib/utils/tryCatch';
import { getFileLanguage } from '$lib/components/views/File/Editor/utils/getFileLanguage';
import { openAgentDiffTab } from './openAgentDiffTab';
import { registerDiffApproval, clearDiffApproval } from '../diffApprovals';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import type { OnApprovalRequested } from '$lib/components/views/Chat/composables/useToolExecution.svelte';

const inputSchema = z.object({
	uri: z
		.string()
		.describe(
			'Absolute file path; use files[].uri from get_chat_context(). Never hardcode paths.'
		),
	content: z
		.string()
		.describe('Complete file content. This replaces the entire file; never pass partial content.')
});

const outputSchema = z.union([
	z.object({
		error: z.string().describe('Error message; surface to user, do not proceed'),
		success: z.literal(false)
	}),
	z.object({ success: z.literal(true) })
]);

export const editFileDef = toolDefinition({
	name: 'edit_file',
	description: `Replaces the entire content of a file, or creates it if it does not exist.
Opens a diff view for the user to review (Original vs Modified). Use Allow to apply or Deny to cancel.

IMPORTANT: This tool replaces the ENTIRE file content. It is not a patch.
Always call read_file first, incorporate your changes into the full existing
content, then pass the complete result here. Passing partial content will
overwrite and lose everything else in the file.

IMPORTANT: never write any comment. 

Use files[].uri from get_chat_context() as the uri. Never hardcode paths.
If error is returned, surface it to the user and do not proceed.`,
	needsApproval: true,
	inputSchema,
	outputSchema
});

type ImplArgs = z.infer<typeof inputSchema>;

async function titleFromUri(uri: string): Promise<string> {
	const segment = uri.split('/').filter(Boolean).pop();
	if (!segment) return 'New file';
	const [r, err] = await tryCatch(decodeURIComponent, segment);
	if (err) return segment;
	return r;
}

// Session data so the executor can find the right diff tab and write the correct file.
type EditFileSession = { uri: string; isTemp: boolean };
const sessions = new Map<string, EditFileSession>();

function findDiffTabByToolCallId(toolCallId: string) {
	return getAllGroups()
		.flatMap((g) => g.tabs)
		.find((t) => (t.diff?.meta as { toolCallId?: string })?.toolCallId === toolCallId);
}

function cancelDiff(toolCallId: string): void {
	const diffTab = findDiffTabByToolCallId(toolCallId);
	if (diffTab) removeTab(diffTab.id);
	sessions.delete(toolCallId);
	clearDiffApproval(toolCallId);
}

async function applyDiffAndClose(
	toolCallId: string
): Promise<{ success: true } | { success: false; error: string }> {
	const session = sessions.get(toolCallId);
	if (!session) return { success: false, error: 'Session not found.' };

	const diffTab = findDiffTabByToolCallId(toolCallId);
	const newContent = diffTab?.diff?.panels?.[1]?.content ?? '';

	if (session.isTemp) {
		const targetTab = getAllGroups()
			.flatMap((g) => g.tabs)
			.find((t) => t.uri === session.uri);
		if (!targetTab?.file) {
			notifyError('Temp file tab not found.');
			if (diffTab) removeTab(diffTab.id);
			sessions.delete(toolCallId);
			return { success: false, error: 'Temp file tab not found.' };
		}
		updateTab({ ...targetTab, file: { ...targetTab.file, content: newContent } });
	} else {
		const [, err] = await tryCatch(fs.Write, { uri: session.uri, content: newContent });
		if (err) {
			const msg = err instanceof Error ? err.message : String(err);
			notifyError(msg);
			if (diffTab) removeTab(diffTab.id);
			sessions.delete(toolCallId);
			return { success: false, error: msg };
		}
	}

	if (diffTab) removeTab(diffTab.id);
	sessions.delete(toolCallId);
	clearDiffApproval(toolCallId);
	return { success: true };
}

/** Factory: returns the onApprovalRequested handler for edit_file, bound to a chat session. */
export function createEditFileApprovalHandler(sessionId: string): OnApprovalRequested {
	return async (toolCallId, rawArgs, { approve, deny }) => {
		const { uri, content } = rawArgs as ImplArgs;
		const isTemp = uri.startsWith('temp://');

		let originalContent = '';
		if (isTemp) {
			const tab = getAllGroups()
				.flatMap((g) => g.tabs)
				.find((t) => t.uri === uri);
			if (!tab?.file) return;
			originalContent = tab.file.content ?? '';
		} else {
			const [onDiskContent, err] = await tryCatch(fs.ReadFile, { uri });
			if (!err && onDiskContent != null) originalContent = onDiskContent;
		}

		sessions.set(toolCallId, { uri, isTemp });

		// Wrap deny to also close the diff tab
		registerDiffApproval(toolCallId, {
			approve,
			deny: async () => {
				cancelDiff(toolCallId);
				await deny();
			}
		});

		openAgentDiffTab({
			uri,
			originalContent,
			newContent: content,
			isTemp,
			language: getFileLanguage(await titleFromUri(uri)),
			toolCallId,
			sessionId
		});
	};
}

/** Executor: runs after approval; reads the (possibly edited) diff content and writes the file. */
async function editFileImpl(
	_args: unknown,
	ctx?: { toolCallId: string }
): Promise<{ success: true } | { success: false; error: string }> {
	if (!ctx) return { success: false, error: 'edit_file requires execution context.' };
	return applyDiffAndClose(ctx.toolCallId);
}

export const editFileClient = editFileDef.client();
export const editFileExecutor = editFileImpl;
