<script lang="ts">
	import type { ToolCallPart } from '$lib/components/views/Chat/core';
	import { getToolTitleParts } from '../utils/getToolTitle';

	type Props = {
		toolCall: ToolCallPart;
	};

	let { toolCall }: Props = $props();

	const title = $derived(getToolTitleParts(toolCall.name, toolCall.arguments));
</script>

<span class="tool-card-title">
	{title.prefix}
	{#if title.code}
		<code>{title.code}</code>
	{/if}
	{#if title.suffix}
		{title.suffix}
	{/if}
</span>

<style>
	.tool-card-title {
		flex: 1;
	}
	:global(.tool-card-title code) {
		color: var(--gray-800);
	}
</style>
