<script lang="ts">
	import type { UIMessage, ToolCallPart } from '$lib/components/views/Chat/core';
	import MarkdownContent from './MarkdownContent.svelte';
	import ToolCallCard from './ToolCallCard.svelte';
	import ThinkingCard from './ThinkingCard.svelte';

	type Props = {
		message: UIMessage;
		isLast: boolean;
		isStreaming: boolean;
		expandedSections: Set<string>;
		onToggleSection: (key: string) => void;
		onApproveAndRun: (toolCall: ToolCallPart) => Promise<void>;
		onDenyToolCall: (toolCall: ToolCallPart) => void;
	};

	let {
		message,
		isLast,
		isStreaming,
		expandedSections,
		onToggleSection,
		onApproveAndRun,
		onDenyToolCall
	}: Props = $props();

	function stripContextPrefix(text: string): string {
		return text.replace(/^<context>[\s\S]*?<\/context>\n\n/, '');
	}

	function partKey(part: (typeof message.parts)[number], partIdx: number): string {
		if (part.type === 'tool-call') {
			return `${message.id}-${partIdx}-${part.state ?? ''}-${part.approval?.id ?? ''}`;
		}
		return `${message.id}-${partIdx}-${part.type}`;
	}
</script>

<svelte:element
	this={message.role === 'user' ? 'article' : 'div'}
	class="message"
	class:user={message.role === 'user'}
	class:assistant={message.role === 'assistant'}
>
	<div class="content" class:markdown={message.role === 'assistant'}>
		{#each message.parts ?? [] as part, partIdx (partKey(part, partIdx))}
			{#if part.type === 'text'}
				{#if message.role === 'assistant'}
					<MarkdownContent content={String(part.content ?? '')} />
				{:else}
					<p class="selectable">{stripContextPrefix(String(part.content ?? ''))}</p>
				{/if}
			{:else if part.type === 'tool-call'}
				{@const key = `${message.id}-${partIdx}`}
				{@const expanded = expandedSections.has(key)}
				{@const tc = part as ToolCallPart}
				<ToolCallCard
					toolCall={tc}
					{expanded}
					onToggle={() => onToggleSection(key)}
					onApprove={() => onApproveAndRun(part)}
					onDeny={() => onDenyToolCall(part)}
				/>
			{:else if part.type === 'thinking'}
				{@const key = `${message.id}-think-${partIdx}`}
				{@const expanded = expandedSections.has(key)}
				<ThinkingCard
					content={String(part.content ?? '')}
					{expanded}
					onToggle={() => onToggleSection(key)}
				/>
			{/if}
		{/each}
	</div>
</svelte:element>
{#if isLast && isStreaming}
	<span class="streaming-cursor" aria-hidden="true">▌</span>
{/if}

<style>
	.message {
		max-width: 90%;
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
		border-radius: var(--br-xs);
		margin: 0 var(--space-sm-md);
	}

	.message.user {
		align-self: flex-end;
		border: var(--border);
		background-color: var(--gray-400);
		padding: var(--space-xs-sm) var(--space-sm);
	}

	.message.assistant {
		align-self: flex-start;
	}

	.message.user .content :global(p) {
		color: var(--gray-900);
		white-space: pre-wrap;
		font-size: var(--fs-md);
		line-height: 20px;
	}

	.content {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.content.markdown {
		white-space: normal;
	}

	.streaming-cursor {
		display: inline-block;
		animation: blink 1s step-end infinite;
	}

	@keyframes blink {
		50% {
			opacity: 0;
		}
	}
</style>
