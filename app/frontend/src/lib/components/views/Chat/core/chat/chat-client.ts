import type {
	AddToolResultParams,
	AnyClientTool,
	ChatClientState,
	ClientTool,
	ConnectionAdapter,
	MessagePart,
	MultimodalContent,
	ToolCallPart,
	UIMessage
} from './types';
import type { StreamChunk } from '../llm';
import { generateMessageId } from './messages';

export interface ChatClientOptions {
	connection: ConnectionAdapter;
	id?: string;
	initialMessages?: UIMessage[];
	body?: Record<string, unknown>;
	tools?: ReadonlyArray<AnyClientTool>;
	onChunk?: (chunk: StreamChunk) => void;
	onFinish?: (message: UIMessage) => void;
	onError?: (error: Error) => void;
	onResponse?: () => void | Promise<void>;
	onMessagesChange?: (messages: UIMessage[]) => void;
	onLoadingChange?: (isLoading: boolean) => void;
	onStatusChange?: (status: ChatClientState) => void;
	onErrorChange?: (error: Error | undefined) => void;
}

function parseArgs(json: string): unknown {
	try {
		return json ? JSON.parse(json) : {};
	} catch {
		return {};
	}
}

/**
 * Cap on consecutive turns that produce nothing usable (empty / whitespace /
 * reasoning-only). Productive turns reset the counter, so this bounds only
 * runaway "the model keeps returning nothing" loops, not legitimate long tool
 * chains.
 */
const MAX_IDLE_TURNS = 3;

/**
 * Owns the conversation: streams assistant turns from the connection, reduces
 * chunks into message parts, runs auto-executable tools, and re-invokes the
 * model once every tool call in the last turn has a result (the agent loop).
 */
export class ChatClient {
	private connection: ConnectionAdapter;
	private readonly uniqueId: string;
	private body: Record<string, unknown>;
	private tools = new Map<string, ClientTool>();

	private messages: UIMessage[];
	private isLoading = false;
	private status: ChatClientState = 'ready';
	private error: Error | undefined;

	private abortController: AbortController | null = null;
	/** True while the multi-turn agent loop is running (guards re-entry). */
	private loopRunning = false;
	/** Whether the most recent streamed turn produced a surviving, usable message. */
	private lastTurnProductive = false;
	private currentAssistantId: string | null = null;

	private onChunk?: (chunk: StreamChunk) => void;
	private onFinish?: (message: UIMessage) => void;
	private onErrorCb?: (error: Error) => void;
	private onResponse?: () => void | Promise<void>;
	private readonly onMessagesChange?: (messages: UIMessage[]) => void;
	private readonly onLoadingChange?: (isLoading: boolean) => void;
	private readonly onStatusChange?: (status: ChatClientState) => void;
	private readonly onErrorChange?: (error: Error | undefined) => void;

	constructor(options: ChatClientOptions) {
		this.connection = options.connection;
		this.uniqueId = options.id ?? `chat-${Date.now()}-${Math.random().toString(36).substring(7)}`;
		this.body = options.body ?? {};
		this.messages = options.initialMessages ? [...options.initialMessages] : [];
		for (const tool of options.tools ?? []) this.tools.set(tool.name, tool);

		this.onChunk = options.onChunk;
		this.onFinish = options.onFinish;
		this.onErrorCb = options.onError;
		this.onResponse = options.onResponse;
		this.onMessagesChange = options.onMessagesChange;
		this.onLoadingChange = options.onLoadingChange;
		this.onStatusChange = options.onStatusChange;
		this.onErrorChange = options.onErrorChange;
	}

	// ---- state setters (broadcast to the reactive wrapper) --------------------

	private emitMessages() {
		this.onMessagesChange?.([...this.messages]);
	}
	private setLoading(v: boolean) {
		if (this.isLoading === v) return;
		this.isLoading = v;
		this.onLoadingChange?.(v);
	}
	private setStatus(v: ChatClientState) {
		if (this.status === v) return;
		this.status = v;
		this.onStatusChange?.(v);
	}
	private setError(v: Error | undefined) {
		this.error = v;
		this.onErrorChange?.(v);
	}

	// ---- public API -----------------------------------------------------------

	async sendMessage(content: string | MultimodalContent): Promise<void> {
		const text = typeof content === 'string' ? content : content.content;
		if (typeof content === 'string' && !text.trim()) return;
		if (this.loopRunning) return;

		this.messages = [
			...this.messages,
			{
				id: typeof content === 'string' ? generateMessageId() : (content.id ?? generateMessageId()),
				role: 'user',
				parts: [{ type: 'text', content: text }],
				createdAt: new Date()
			}
		];
		this.emitMessages();
		await this.runTurns();
	}

	async append(message: UIMessage): Promise<void> {
		if (message.role === 'system') return;
		this.messages = [...this.messages, message];
		this.emitMessages();
		if (!this.loopRunning) await this.runTurns();
	}

	async reload(): Promise<void> {
		if (this.loopRunning) return;
		const lastUser = this.messages.map((m) => m.role).lastIndexOf('user');
		if (lastUser === -1) return;
		this.messages = this.messages.slice(0, lastUser + 1);
		this.emitMessages();
		await this.runTurns();
	}

	stop(): void {
		this.abortController?.abort();
		this.abortController = null;
		this.setLoading(false);
		this.setStatus('ready');
	}

	clear(): void {
		this.messages = [];
		this.currentAssistantId = null;
		this.setError(undefined);
		this.emitMessages();
	}

	setMessagesManually(messages: UIMessage[]): void {
		this.messages = [...messages];
		this.emitMessages();
	}

	updateOptions(options: { body?: Record<string, unknown>; tools?: ReadonlyArray<AnyClientTool> }): void {
		if (options.body) this.body = options.body;
		if (options.tools) {
			this.tools = new Map(options.tools.map((t) => [t.name, t as ClientTool]));
		}
	}

	async addToolResult(result: AddToolResultParams): Promise<void> {
		this.recordToolResult(result.toolCallId, result.output, result.errorText);
		this.emitMessages();
		await this.maybeContinue();
	}

	async addToolApprovalResponse(response: { id: string; approved: boolean }): Promise<void> {
		this.patchToolCallByApprovalId(response.id, (part) => {
			part.state = 'approval-responded';
			if (part.approval) part.approval = { ...part.approval, approved: response.approved };
		});
		this.emitMessages();
		await this.maybeContinue();
	}

	// ---- request lifecycle ----------------------------------------------------

	/**
	 * Drive consecutive model turns until the model gives a final answer, a tool
	 * is waiting on an external result, or too many turns produce nothing usable.
	 * Iterative (not recursive) so an empty turn can re-prompt without tripping a
	 * re-entrancy guard.
	 */
	private async runTurns(): Promise<void> {
		if (this.loopRunning) return;
		this.loopRunning = true;
		try {
			let idleTurns = 0;
			for (;;) {
				const productive = await this.streamOneTurn();
				if (this.status === 'error') break;

				const last = this.lastAssistant();
				const toolCalls =
					last?.parts.filter((p): p is ToolCallPart => p.type === 'tool-call') ?? [];

				// A tool call is still waiting on its result (a manual/approval tool):
				// pause here — addToolResult/addToolApprovalResponse resumes the loop.
				if (toolCalls.length > 0 && !this.areAllToolsComplete(toolCalls)) break;

				if (productive) {
					idleTurns = 0;
					// A final answer with no tools to react to: we're done.
					if (toolCalls.length === 0) break;
					// Otherwise every tool call is resolved — loop so the model reacts.
					continue;
				}

				// Unproductive turn (empty / whitespace / reasoning-only, now removed).
				// Retry only while there is a completed tool the model still owes an
				// answer for, and only up to the idle cap so it can't spin forever.
				if (++idleTurns >= MAX_IDLE_TURNS) break;
				if (toolCalls.length === 0) break;
			}
		} finally {
			this.loopRunning = false;
		}
	}

	/** Resume the loop after an externally-supplied tool result completes a turn. */
	private async maybeContinue(): Promise<void> {
		if (this.loopRunning) return;
		const last = this.lastAssistant();
		const toolCalls = last?.parts.filter((p): p is ToolCallPart => p.type === 'tool-call') ?? [];
		if (toolCalls.length === 0) return;
		if (!this.areAllToolsComplete(toolCalls)) return;
		await this.runTurns();
	}

	/** Stream a single assistant turn. Returns whether it produced a usable message. */
	private async streamOneTurn(): Promise<boolean> {
		this.lastTurnProductive = false;
		this.setLoading(true);
		this.setStatus('submitted');
		this.setError(undefined);
		const ac = new AbortController();
		this.abortController = ac;

		try {
			await this.onResponse?.();
			const data = { ...this.body, conversationId: this.uniqueId };
			const source = this.connection.connect(this.messages, data, ac.signal);
			await this.processStream(source);
			await this.runAutoExecutableTools();
		} catch (err) {
			if (!ac.signal.aborted && (err as Error)?.name !== 'AbortError') {
				const e = err instanceof Error ? err : new Error(String(err));
				this.setError(e);
				this.setStatus('error');
				this.onErrorCb?.(e);
			}
		} finally {
			if (this.abortController === ac) this.abortController = null;
			this.setLoading(false);
			if (this.status !== 'error') this.setStatus('ready');
		}

		return this.lastTurnProductive;
	}

	private async processStream(source: AsyncIterable<StreamChunk>): Promise<void> {
		this.currentAssistantId = null;

		for await (const chunk of source) {
			this.onChunk?.(chunk);
			this.handleChunk(chunk);
		}

		this.finalizeStream();
	}

	private handleChunk(chunk: StreamChunk): void {
		switch (chunk.type) {
			case 'text':
				this.ensureAssistant();
				this.appendText(chunk.delta);
				break;
			case 'thinking':
				this.ensureAssistant();
				this.appendThinking(chunk.delta);
				break;
			case 'tool-start':
				this.ensureAssistant();
				this.patchAssistant((parts) => {
					parts.push({
						type: 'tool-call',
						id: chunk.id,
						name: chunk.name,
						arguments: '',
						state: 'awaiting-input'
					});
				});
				break;
			case 'tool-args':
				this.patchAssistant((parts) => {
					const part = parts.find(
						(p): p is ToolCallPart => p.type === 'tool-call' && p.id === chunk.id
					);
					if (part) {
						part.arguments += chunk.delta;
						part.state = 'input-streaming';
					}
				});
				break;
			case 'error':
				this.setError(new Error(chunk.message));
				this.setStatus('error');
				this.onErrorCb?.(new Error(chunk.message));
				break;
			case 'finish':
				break;
		}
	}

	private finalizeStream(): void {
		const msg = this.currentAssistant();
		if (!msg) return;

		// Finalize any tool calls and decide which ones need approval.
		this.patchAssistant((parts) => {
			for (const part of parts) {
				if (part.type !== 'tool-call') continue;
				if (part.state === 'awaiting-input' || part.state === 'input-streaming') {
					part.state = 'input-complete';
				}
				if (part.state === 'input-complete' && this.tools.get(part.name)?.needsApproval) {
					part.state = 'approval-requested';
					part.approval = { id: `approval-${part.id}`, needsApproval: true };
				}
			}
		});

		const finalized = this.currentAssistant();
		const parts = finalized?.parts ?? [];
		const hasToolCall = parts.some((p) => p.type === 'tool-call');
		const hasVisibleText = parts.some(
			(p) => p.type === 'text' && (p as { content: string }).content.trim().length > 0
		);

		// Drop a turn with nothing usable — empty, whitespace-only (e.g. a provider
		// emitting "\n"), or reasoning-only — so runTurns can re-prompt instead of
		// leaving the user staring at an empty/thinking-only bubble.
		if (!hasToolCall && !hasVisibleText && this.status !== 'error') {
			this.messages = this.messages.filter((m) => m.id !== this.currentAssistantId);
			this.currentAssistantId = null;
			this.emitMessages();
			return;
		}

		this.lastTurnProductive = true;
		if (finalized) this.onFinish?.(finalized);
	}

	/** Run client tools that carry an `execute` fn and don't need approval. */
	private async runAutoExecutableTools(): Promise<void> {
		const msg = this.lastAssistant();
		if (!msg) return;

		const runnable = msg.parts.filter(
			(p): p is ToolCallPart =>
				p.type === 'tool-call' &&
				p.state === 'input-complete' &&
				p.output === undefined &&
				typeof this.tools.get(p.name)?.execute === 'function'
		);

		for (const part of runnable) {
			const tool = this.tools.get(part.name)!;
			try {
				const output = await tool.execute!(parseArgs(part.arguments));
				this.recordToolResult(part.id, output);
			} catch (err) {
				const message = err instanceof Error ? err.message : String(err);
				this.recordToolResult(part.id, { error: message, success: false }, message);
			}
			this.emitMessages();
		}
	}

	private areAllToolsComplete(toolCalls: ToolCallPart[]): boolean {
		const resultIds = this.toolResultIds();
		return toolCalls.every(
			(tc) =>
				tc.state === 'approval-responded' ||
				(tc.output !== undefined && !tc.approval) ||
				resultIds.has(tc.id)
		);
	}

	// ---- message/part mutation helpers ---------------------------------------

	private ensureAssistant(): void {
		if (this.currentAssistantId) return;
		const message: UIMessage = {
			id: generateMessageId(),
			role: 'assistant',
			parts: [],
			createdAt: new Date()
		};
		this.currentAssistantId = message.id;
		this.messages = [...this.messages, message];
		this.setStatus('streaming');
		this.emitMessages();
	}

	private currentAssistant(): UIMessage | undefined {
		return this.messages.find((m) => m.id === this.currentAssistantId);
	}

	/** Latest assistant message in the conversation (survives remounts, unlike the stream id). */
	private lastAssistant(): UIMessage | undefined {
		for (let i = this.messages.length - 1; i >= 0; i--) {
			if (this.messages[i].role === 'assistant') return this.messages[i];
		}
		return undefined;
	}

	/** Replace the current assistant message with a fresh copy after mutating its parts. */
	private patchAssistant(mutate: (parts: MessagePart[]) => void): void {
		const idx = this.messages.findIndex((m) => m.id === this.currentAssistantId);
		if (idx === -1) return;
		const parts = this.messages[idx].parts.map((p) => ({ ...p }) as MessagePart);
		mutate(parts);
		const next = { ...this.messages[idx], parts };
		this.messages = [...this.messages.slice(0, idx), next, ...this.messages.slice(idx + 1)];
		this.emitMessages();
	}

	private appendText(delta: string): void {
		this.patchAssistant((parts) => {
			const last = parts[parts.length - 1];
			if (last && last.type === 'text') last.content += delta;
			else parts.push({ type: 'text', content: delta });
		});
	}

	private appendThinking(delta: string): void {
		this.patchAssistant((parts) => {
			const existing = parts.find((p) => p.type === 'thinking');
			if (existing) (existing as { content: string }).content += delta;
			else parts.push({ type: 'thinking', content: delta });
		});
	}

	/** Set a tool call's output and append its tool-result part (idempotent). */
	private recordToolResult(toolCallId: string, output: unknown, errorText?: string): void {
		const idx = this.messages.findIndex((m) =>
			m.parts.some((p) => p.type === 'tool-call' && p.id === toolCallId)
		);
		if (idx === -1) return;

		const value = errorText ? { error: errorText } : output;
		const content = typeof value === 'string' ? value : JSON.stringify(value);

		const parts = this.messages[idx].parts.map((p) => {
			if (p.type === 'tool-call' && p.id === toolCallId) {
				const next: ToolCallPart = { ...p, output: value };
				if (next.state !== 'approval-responded') next.state = 'input-complete';
				return next;
			}
			return { ...p } as MessagePart;
		});

		const hasResult = parts.some((p) => p.type === 'tool-result' && p.toolCallId === toolCallId);
		if (!hasResult) {
			parts.push({
				type: 'tool-result',
				toolCallId,
				content,
				state: errorText ? 'error' : 'complete',
				...(errorText ? { error: errorText } : {})
			});
		}

		const next = { ...this.messages[idx], parts };
		this.messages = [...this.messages.slice(0, idx), next, ...this.messages.slice(idx + 1)];
	}

	private patchToolCallByApprovalId(approvalId: string, mutate: (part: ToolCallPart) => void): void {
		const idx = this.messages.findIndex((m) =>
			m.parts.some((p) => p.type === 'tool-call' && p.approval?.id === approvalId)
		);
		if (idx === -1) return;
		const parts = this.messages[idx].parts.map((p) => {
			if (p.type === 'tool-call' && p.approval?.id === approvalId) {
				const next = { ...p } as ToolCallPart;
				mutate(next);
				return next;
			}
			return { ...p } as MessagePart;
		});
		const next = { ...this.messages[idx], parts };
		this.messages = [...this.messages.slice(0, idx), next, ...this.messages.slice(idx + 1)];
	}

	private toolResultIds(): Set<string> {
		const ids = new Set<string>();
		for (const m of this.messages) {
			for (const p of m.parts) if (p.type === 'tool-result') ids.add(p.toolCallId);
		}
		return ids;
	}

	// ---- getters --------------------------------------------------------------

	getMessages(): UIMessage[] {
		return this.messages;
	}
}
