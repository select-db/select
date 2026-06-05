<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';

	import { createChat } from '$lib/components/views/Chat/core';
	import {
		createUnifiedConnection,
		getProvider,
		getModelId,
		hasApiKey,
		getPreferredModelWithApiKey,
		hasAnyWorkspaceApiKey
	} from '$lib/components/views/Chat/core/aiConnections';
	import { createChatTools, getContextForChatTab } from '$lib/components/views/Chat/tools';
	import { buildContext } from '$lib/components/views/Chat/utils/buildContext';
	import { tryCatch, must } from '$lib/utils/tryCatch';
	import { updateTab, type Tab } from '$lib/components/Layout/layoutStore';
	import type { UIMessage } from '$lib/components/views/Chat/core';

	import {
		createSystemMessage,
		TASK_TRIGGER_MESSAGE,
		API_KEY_ENV,
		DEFAULT_MODEL
	} from './utils/constants';

	import { useToolExecution } from './composables/useToolExecution.svelte.ts';
	import { useChatPersistence } from './composables/useChatPersistence.svelte.ts';
	import { useChatScroll } from './composables/useChatScroll.svelte.ts';

	import EmptyState from './components/EmptyState.svelte';
	import StatusIndicator from './components/StatusIndicator.svelte';
	import MessageBubble from './components/MessageBubble.svelte';
	import ChatFooter from './components/ChatFooter.svelte';

	import { createQueryRegistry } from './tools/query/queryRegistry.ts';
	import Alert from '$lib/system/Alert/Alert.svelte';
	import { AlertType } from '$lib/system/Alert/types.ts';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();

	const sessionId = tab.chat?.sessionId ?? tab.id;

	const restoredMessages = (tab.chat?.messages ?? []).map((m) => ({
		id: m.id ?? crypto.randomUUID(),
		role: m.role as 'system' | 'user' | 'assistant',
		parts: m.parts ?? []
	})) as UIMessage[];

	const task = tab.chat?.task;
	const initialMessages = [createSystemMessage(task), ...restoredMessages];
	const initialModel = tab.chat?.selectedModel ?? DEFAULT_MODEL;
	const initialInput = tab.chat?.inputValue ?? '';

	let selectedValue = $state(initialModel);
	let inputText = $state(initialInput);

	const queryRegistry = createQueryRegistry();
	const { tools, toolExecutors, onApprovalRequestedHandlers } = createChatTools(
		sessionId,
		queryRegistry
	);
	const chat = createChat({
		id: sessionId,
		connection: createUnifiedConnection(tools),
		tools,
		initialMessages,
		body: (() => {
			const p = getProvider(initialModel);
			const m = getModelId(initialModel);
			return { provider: p, model: m };
		})()
	});

	const { handleApproveAndRunTool, handleDenyToolCall, denyAllPendingApprovals } = useToolExecution(
		chat,
		toolExecutors,
		onApprovalRequestedHandlers,
		stopChatInternal
	);

	async function stopChatInternal() {
		await queryRegistry.cancelAll();
		chat.stop();
	}

	async function stopChat() {
		denyAllPendingApprovals();
		await stopChatInternal();
	}

	$effect(() => {
		chat.updateBody({ provider: getProvider(selectedValue), model: getModelId(selectedValue) });
	});

	let messagesEl: HTMLDivElement | undefined;
	const scroll = useChatScroll(tab, {
		getScrollEl: () => messagesEl,
		getIsStreaming: () => lastMessageStreaming,
		getMessages: () => chat.messages
	});

	onMount(() => {
		scroll.restore();

		// When no model was persisted, preselect first provider that has an API key set
		if (!tab.chat?.selectedModel) {
			getPreferredModelWithApiKey().then((model) => {
				selectedValue = model;
			});
		}

		const taskPayload = tab.chat?.task;
		if (!taskPayload || typeof taskPayload !== 'string') return;

		const taskBlock = taskPayload;
		updateTab({
			...tab,
			chat: { ...tab.chat!, task: undefined }
		});

		(async () => {
			await must(
				tryCatch(async () => {
					const context = await buildContext(getContextForChatTab(tab));
					const systemWithTaskAndContext = createSystemMessage(
						context ? `${taskBlock}\n\n${context}` : taskBlock
					);
					chat.setMessages([systemWithTaskAndContext, ...chat.messages.slice(1)]);
					await tick();
					await chat.sendMessage(TASK_TRIGGER_MESSAGE);
					await scroll.scrollToBottom();
				})
			);
		})();
	});

	onDestroy(async () => await stopChat());

	useChatPersistence(
		chat,
		() => tab,
		sessionId,
		() => inputText,
		() => selectedValue,
		() => scroll.scrollTop
	);

	let expandedSections = $state<Set<string>>(new Set());

	function toggleSection(key: string) {
		expandedSections = new Set(expandedSections);
		if (expandedSections.has(key)) expandedSections.delete(key);
		else expandedSections.add(key);
	}

	const sending = $derived(chat.isLoading);
	const statusLabel = $derived(
		chat.status === 'submitted' ? 'Sending…' : chat.status === 'streaming' ? 'Generating…' : null
	);

	function hasVisibleText(msg: (typeof chat.messages)[0]): boolean {
		if (msg.role !== 'user') return true;
		const textPart = msg.parts?.find((p) => (p as { type?: string }).type === 'text') as
			| { content?: string }
			| undefined;
		const text = typeof textPart?.content === 'string' ? textPart.content.trim() : '';
		// Hide empty/invisible content and the task trigger (so user doesn't see the auto-sent prompt)
		if (text.replace(/\s|\u200B/g, '').length === 0) return false;
		if (text === TASK_TRIGGER_MESSAGE) return false;
		return true;
	}
	const visibleMessages = $derived(
		chat.messages.filter((m) => m.role !== 'system' && hasVisibleText(m))
	);
	const lastMsg = $derived(chat.messages[chat.messages.length - 1]);
	const lastMessageStreaming = $derived(
		sending && (chat.status === 'streaming' ? lastMsg?.role === 'assistant' : true)
	);

	let localError = $state<string | null>(null);
	const displayError = $derived(localError ?? chat.error?.message ?? null);

	let workspaceHasApiKey = $state<boolean | null>(null);

	$effect(() => {
		void (async () => {
			workspaceHasApiKey = await hasAnyWorkspaceApiKey();
		})();
	});

	async function handleSend() {
		const text = inputText.trim();

		if (!text) return;

		// Deny all pending approvals when user sends a new message
		denyAllPendingApprovals();

		if (sending) await stopChatInternal();

		const provider = getProvider(selectedValue);

		if (!(await hasApiKey(provider))) {
			const envVar = API_KEY_ENV[provider] ?? 'API key';
			localError = `Set ${envVar} in .env for this provider.`;
			return;
		}

		localError = null;
		inputText = '';

		const context = await buildContext(getContextForChatTab(tab));
		await chat.sendMessage(context ? `${context}\n\n${text}` : text);

		await scroll.scrollToBottom();
	}
</script>

<div class="chat">
	<div
		class="messages scrollable"
		bind:this={messagesEl}
		onscroll={scroll.onScroll}
		role="region"
		aria-label="Chat messages"
	>
		{#if !lastMessageStreaming && visibleMessages.length === 0 && workspaceHasApiKey === false}
			<EmptyState />
		{:else}
			{#each visibleMessages as message (message.id)}
				<MessageBubble
					{message}
					{expandedSections}
					onApproveAndRun={handleApproveAndRunTool}
					onDenyToolCall={handleDenyToolCall}
					isLast={message === lastMsg}
					isStreaming={lastMessageStreaming && message === lastMsg}
					onToggleSection={toggleSection}
				/>
			{/each}
			<StatusIndicator status={statusLabel} />
		{/if}
		{#if displayError}
			<Alert
				type={AlertType.Error}
				message={displayError}
				style="margin-left: 0;"
				copyable
				noPulse
			/>
		{/if}
	</div>

	<ChatFooter
		bind:inputText
		bind:selectedModel={selectedValue}
		{sending}
		{tab}
		onSend={handleSend}
		onStop={stopChat}
	/>
</div>

<style>
	.chat {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		background-color: var(--gray-0);
	}

	.messages {
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden;

		display: flex;
		flex-direction: column;
		gap: var(--space-md);

		padding: var(--space-sm-md) var(--space-md) var(--space-sm-md) var(--space-md);
		min-width: 275px;
	}
</style>
