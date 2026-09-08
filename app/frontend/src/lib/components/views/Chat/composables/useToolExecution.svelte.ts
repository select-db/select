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
 * Tool call IDs are unique per invocation — a provider that does not send one gets a
 * generated id — so there is no collision risk across sessions.
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

/** What an executor came back with, ready to be handed to a chat. */
type ToolOutcome = { output: unknown; state: 'output-available' | 'output-error' };

/**
 * runs: the executor run for a tool call, keyed by call id, module-level so one run
 * covers every path that asks for it — the approval callbacks, the direct
 * approve-and-run, and the safety net below.
 *
 * An entry stays until its outcome has been written to a live chat. A component
 * destroyed mid-run (a tab switch unmounts Chat) leaves its run here rather than
 * recording into a client nobody reads, and the instance that replaces it adopts
 * the outcome instead of leaving the call's card spinning for good.
 */
const runs = new Map<string, Promise<ToolOutcome>>();

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
	/** Tool calls this instance is already waiting on, so effect re-runs don't pile up. */
	const awaiting = new Set<string>();
	/** False once this component is torn down; a run that settles after that is not ours to record. */
	let alive = true;
	$effect(() => () => {
		alive = false;
	});

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

	async function execute(tc: ToolCallPart, args: unknown): Promise<ToolOutcome> {
		const [result, err] = await tryCatch(toolExecutors[tc.name], args, buildContext(tc));
		return err
			? { output: { error: err.message, success: false }, state: 'output-error' }
			: { output: result, state: 'output-available' };
	}

	/** Ends a call the app cannot run, so the model hears about it and the card stops. */
	async function reportUnrunnable(tc: ToolCallPart, error: string): Promise<void> {
		if (awaiting.has(tc.id)) return;
		awaiting.add(tc.id);
		try {
			if (messageHasToolResultFor(chat.messages, tc.id)) return;
			await chat.addToolResult({
				toolCallId: tc.id,
				tool: tc.name,
				output: { error, success: false },
				state: 'output-error'
			});
		} finally {
			awaiting.delete(tc.id);
		}
	}

	async function runExecutor(tc: ToolCallPart, args: unknown): Promise<void> {
		// One execution per tool call. Approving flips the state to
		// 'approval-responded', which is exactly what the safety-net effect below
		// looks for, and the result is not recorded until the run settles — so both
		// paths reach here. The second one waits on the first run rather than
		// starting its own over the top of it (edit_file has consumed its diff
		// session by then, and would report a failure over a successful edit).
		if (awaiting.has(tc.id)) return;
		awaiting.add(tc.id);

		try {
			let run = runs.get(tc.id);
			if (!run) {
				run = execute(tc, args);
				runs.set(tc.id, run);
			}

			const outcome = await run;
			// This chat is gone — the component was destroyed while the tool ran.
			// Leave the run for whichever instance takes over, so the result is not
			// dropped into a client nothing renders.
			if (!alive) return;

			if (!messageHasToolResultFor(chat.messages, tc.id)) {
				await chat.addToolResult({ toolCallId: tc.id, tool: tc.name, ...outcome });
			}
			runs.delete(tc.id);
		} finally {
			awaiting.delete(tc.id);
		}
	}

	$effect(() => {
		const toolCallParts = chat.messages.flatMap((m) => m.parts ?? []).filter(isToolCallPart);
		for (const tc of toolCallParts) {
			if (tc.state !== 'approval-requested') continue;

			const [args, parseErr] = tryCatch(parseArgs, tc.arguments);
			if (parseErr) {
				// Nothing to show an approval dialog for, and nothing to run.
				reportUnrunnable(tc, `Arguments for ${tc.name} were not valid JSON; send them again.`);
				continue;
			}

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
	//
	// Held off while a turn is in flight, so this does not race with the client
	// running the tools that carry their own execute fn. That is what the guard
	// has to mean — it used to read `status === 'ready'`, and a turn that ended
	// in a provider error leaves the status on 'error' for good, which froze
	// every call that turn had already produced. The model asked for them before
	// the stream broke; they still have to run.
	$effect(() => {
		if (chat.isLoading) return;
		const messages = chat.messages;
		const toolCallParts = messages.flatMap((m) => m.parts ?? []).filter(isToolCallPart);
		for (const tc of toolCallParts) {
			if (tc.state === 'approval-requested') continue;
			if (tc.output !== undefined) continue;
			if (messageHasToolResultFor(messages, tc.id)) continue;
			if (awaiting.has(tc.id)) continue;

			// Everything below is a call this app cannot run. Each one still gets a
			// result: the model can read it and try something else, and the card
			// reaches an end state instead of spinning for the rest of the session.
			if (!toolExecutors[tc.name]) {
				reportUnrunnable(tc, `Unknown tool: ${tc.name}. It is not available in this app.`);
				continue;
			}
			if (!tc.arguments) {
				reportUnrunnable(tc, `No arguments were received for ${tc.name}.`);
				continue;
			}
			const [args, parseErr] = tryCatch(parseArgs, tc.arguments);
			if (parseErr) {
				reportUnrunnable(tc, `Arguments for ${tc.name} were not valid JSON; send them again.`);
				continue;
			}

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
