import type {
	CreateChatReturn,
	ToolCallPart,
	UIMessage
} from '$lib/components/views/Chat/core/chat/types';
import { tryCatch } from '$lib/utils/tryCatch';
import { getDiffApproval, clearDiffApproval } from '$lib/components/views/Chat/tools/diffApprovals';
import { removeTab, getAllGroups } from '$lib/components/Layout/layoutStore';

export type ExecutionContext = {
	toolCallId: string;
	toolName: string;
	addToolResult: (output: unknown) => Promise<void>;
};

type ToolExecutor = (args: unknown, ctx?: ExecutionContext) => Promise<unknown>;

export type OnApprovalRequested = (
	toolCallId: string,
	args: unknown,
	callbacks: { approve: () => Promise<void>; deny: () => Promise<void> }
) => void | Promise<void>;

function isToolCallPart(p: { type: string }): p is ToolCallPart {
	return p.type === 'tool-call';
}

function parseArgs(args: unknown): unknown {
	return typeof args === 'string' ? JSON.parse(args) : args;
}

const CANCELLED_OUTPUT = { success: false, cancelled: true } as const;

/**
 * Mutate messages to mark a tool call as approval-responded (approved=true) WITHOUT calling
 * addToolApprovalResponse, which would trigger checkForContinuation before the executor runs.
 * causing the AI to receive an empty tool result and re-call the same tool.
 */
function applyApprovedState(messages: Array<UIMessage>, toolCallId: string): Array<UIMessage> {
	return messages.map((msg) => {
		const parts = msg.parts ?? [];
		if (!parts.some((p): p is ToolCallPart => p.type === 'tool-call' && p.id === toolCallId))
			return msg;
		return {
			...msg,
			parts: parts.map((p) => {
				if (p.type !== 'tool-call' || p.id !== toolCallId) return p;
				const part = { ...p };
				part.state = 'approval-responded';
				if (part.approval) part.approval = { ...part.approval, approved: true };
				return part;
			})
		};
	});
}

/** Apply denied state to messages without calling addToolResult (which would trigger continuation). */
function applyDeniedToolResult(
	messages: Array<UIMessage>,
	toolCallId: string,
	approvalId: string | undefined,
	output: typeof CANCELLED_OUTPUT
): Array<UIMessage> {
	const content = JSON.stringify(output);
	return messages.map((msg) => {
		const parts = msg.parts ?? [];
		const hasToolCall = parts.some(
			(p): p is ToolCallPart => p.type === 'tool-call' && p.id === toolCallId
		);
		if (!hasToolCall) return msg;

		const newParts = parts.map((p) => {
			if (p.type !== 'tool-call' || p.id !== toolCallId) return p;
			const part = { ...p };
			if (part.approval && approvalId && part.approval.id === approvalId) {
				part.approval = { ...part.approval, approved: false };
			}
			part.state = 'approval-responded';
			part.output = output;
			return part;
		});

		const hasResult = newParts.some(
			(p) => p.type === 'tool-result' && (p as { toolCallId: string }).toolCallId === toolCallId
		);
		const withResult = hasResult
			? newParts
			: [
					...newParts,
					{
						type: 'tool-result' as const,
						toolCallId,
						content,
						state: 'complete' as const
					}
				];

		return { ...msg, parts: withResult };
	});
}

/**
 * Module-level maps so approval state survives component remounts (e.g. when opening a diff tab
 * reshuffles the layout and destroys/remounts Chat).
 *
 * Tool call IDs are unique UUIDs per invocation, so there is no collision risk across sessions.
 * openedIds: prevents calling onApprovalRequested more than once per tool call.
 */
const openedIds = new Set<string>();
/**
 * liveCallbacks: always holds approve/deny bound to the CURRENT chat instance so that the diff UI
 * (opened before a remount) routes back to whichever Chat is currently alive.
 */
const liveCallbacks = new Map<
	string,
	{ approve: () => Promise<void>; deny: () => Promise<void> }
>();

/** IDs we've already started executing (safety net for tools that never got onToolCall). */
const executionStartedIds = new Set<string>();

function messageHasToolResultFor(messages: Array<UIMessage>, toolCallId: string): boolean {
	for (const msg of messages) {
		const parts = msg.parts ?? [];
		if (
			parts.some(
				(p) => p.type === 'tool-result' && (p as { toolCallId: string }).toolCallId === toolCallId
			)
		)
			return true;
	}
	return false;
}

export function useToolExecution(
	chat: CreateChatReturn,
	toolExecutors: Record<string, ToolExecutor>,
	onApprovalRequestedHandlers?: Record<string, OnApprovalRequested>,
	stop?: () => Promise<void>
) {
	function buildContext(tc: ToolCallPart): ExecutionContext {
		return {
			toolCallId: tc.id,
			toolName: tc.name,
			addToolResult: (output) =>
				chat.addToolResult({
					toolCallId: tc.id,
					tool: tc.name,
					output,
					state: 'output-available'
				})
		};
	}

	async function runExecutor(tc: ToolCallPart, args: unknown): Promise<void> {
		const [result, err] = await tryCatch(toolExecutors[tc.name], args, buildContext(tc));
		if (err) {
			await chat.addToolResult({
				toolCallId: tc.id,
				tool: tc.name,
				output: { error: err.message, success: false },
				state: 'output-error'
			});
		} else {
			await chat.addToolResult({
				toolCallId: tc.id,
				tool: tc.name,
				output: result,
				state: 'output-available'
			});
		}
	}

	$effect(() => {
		const toolCallParts = chat.messages.flatMap((m) => m.parts ?? []).filter(isToolCallPart);
		for (const tc of toolCallParts) {
			if (tc.state !== 'approval-requested') continue;

			const args = parseArgs(tc.arguments);

			// Always refresh liveCallbacks with the current chat instance so the diff UI
			// (which may have been opened before a remount) uses the live chat.
			liveCallbacks.set(tc.id, {
				approve: async () => {
					// Mark approval-responded via setMessages (no continuation triggered).
					// Using addToolApprovalResponse would fire checkForContinuation immediately.
					// before runExecutor has a chance to call addToolResult, causing the AI to
					// receive an empty result and re-call the same tool.
					chat.setMessages(applyApprovedState(chat.messages, tc.id));
					await runExecutor(tc, args);
				},
				deny: async () => {
					await stop?.();
					chat.setMessages(
						applyDeniedToolResult(chat.messages, tc.id, tc.approval?.id, CANCELLED_OUTPUT)
					);
				}
			});

			// Only open the UI once; openedIds survives remounts.
			if (openedIds.has(tc.id)) continue;
			openedIds.add(tc.id);

			const handler = onApprovalRequestedHandlers?.[tc.name];
			if (!handler) continue;

			// Delegates that always route through the latest liveCallbacks entry.
			handler(tc.id, args, {
				approve: () => liveCallbacks.get(tc.id)?.approve() ?? Promise.resolve(),
				deny: () => liveCallbacks.get(tc.id)?.deny() ?? Promise.resolve()
			});
		}
	});

	// Safety net: run any tool that has input but no result (e.g. missed by client onToolCall).
	// Only when status is 'ready' so we don't race with client execution during streaming.
	$effect(() => {
		if (chat.status !== 'ready') return;
		const messages = chat.messages;
		const toolCallParts = messages.flatMap((m) => m.parts ?? []).filter(isToolCallPart);
		for (const tc of toolCallParts) {
			if (tc.state === 'approval-requested') continue;
			if (tc.output !== undefined) continue;
			if (messageHasToolResultFor(messages, tc.id)) continue;
			if (!toolExecutors[tc.name]) continue;
			if (executionStartedIds.has(tc.id)) continue;
			if (!tc.arguments) continue;

			executionStartedIds.add(tc.id);
			const args = parseArgs(tc.arguments);
			runExecutor(tc, args);
		}
	});

	async function handleApproveAndRunTool(toolCall: ToolCallPart) {
		if (!toolCall.approval?.id) return;

		// If a diff UI is open for this tool, delegate to it
		const diffHandlers = getDiffApproval(toolCall.id);
		if (diffHandlers) {
			await diffHandlers.approve();
			return;
		}

		// No UI open, run directly (e.g. execute_command).
		// Same pattern: mark approved via setMessages, then addToolResult triggers ONE continuation.
		chat.setMessages(applyApprovedState(chat.messages, toolCall.id));
		await runExecutor(toolCall, parseArgs(toolCall.arguments));
	}

	async function handleDenyToolCall(toolCall: ToolCallPart) {
		const diffHandlers = getDiffApproval(toolCall.id);
		if (diffHandlers) {
			await diffHandlers.deny();
			return;
		}

		await stop?.();
		chat.setMessages(
			applyDeniedToolResult(chat.messages, toolCall.id, toolCall.approval?.id, CANCELLED_OUTPUT)
		);
	}

	/** Find and close any diff tab associated with a tool call */
	function closeDiffTabForToolCall(toolCallId: string): void {
		const diffTab = getAllGroups()
			.flatMap((g) => g.tabs)
			.find((t) => (t.diff?.meta as { toolCallId?: string })?.toolCallId === toolCallId);
		if (diffTab) removeTab(diffTab.id);
		clearDiffApproval(toolCallId);
	}

	/** Deny all pending approval tool calls (used when stopping chat or sending new message) */
	function denyAllPendingApprovals(): void {
		const toolCallParts = chat.messages.flatMap((m) => m.parts ?? []).filter(isToolCallPart);
		const pendingApprovals = toolCallParts.filter((tc) => tc.state === 'approval-requested');

		if (pendingApprovals.length === 0) return;

		let updatedMessages = chat.messages;
		for (const tc of pendingApprovals) {
			closeDiffTabForToolCall(tc.id);
			updatedMessages = applyDeniedToolResult(
				updatedMessages,
				tc.id,
				tc.approval?.id,
				CANCELLED_OUTPUT
			);
			liveCallbacks.delete(tc.id);
		}
		chat.setMessages(updatedMessages);
	}

	return { handleApproveAndRunTool, handleDenyToolCall, denyAllPendingApprovals };
}
