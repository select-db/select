<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { addTerminalTab, addChatTab } from '$lib/components/Layout/layoutStore';
</script>

<div class="wrapper tab-actions">
	<Button leftIcon="chat" label="Open Chat" onclick={() => addChatTab()} iconSize={18} />
	<Button
		leftIcon="terminal"
		label="Open Terminal"
		onclick={() => addTerminalTab()}
		iconSize={18}
	/>
</div>

<style>
	.wrapper {
		display: flex;
		align-items: stretch;
		z-index: 10;
		gap: var(--space-xs-sm);
		padding-bottom: var(--space-xs-sm);
	}

	.wrapper.tab-actions :global(button) {
		height: 100%;
	}
</style>
