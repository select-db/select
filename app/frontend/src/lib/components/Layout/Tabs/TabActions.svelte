<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { addTerminalTab, addChatTab } from '$lib/components/Layout/layoutStore';
</script>

<div class="wrapper tab-actions">
	<Button leftIcon="chat" label="Open Chat" onclick={() => addChatTab()} iconSize={18} noRadius />
	<Button
		leftIcon="terminal"
		label="Open Terminal"
		onclick={() => addTerminalTab()}
		iconSize={18}
		noRadius
	/>
</div>

<style>
	.wrapper {
		position: relative;
		z-index: 1;
		flex-shrink: 0;

		display: flex;
		align-items: stretch;

		background-color: var(--gray-100);
		border-bottom: var(--border);
	}

	.wrapper.tab-actions :global(button) {
		height: 100%;
	}
</style>
