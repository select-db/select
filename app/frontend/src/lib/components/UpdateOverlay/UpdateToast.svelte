<script lang="ts">
	import { fly } from 'svelte/transition';
	import { updateStore } from '$lib/stores/updateStore';
	import Button from '$lib/system/Button/Button.svelte';
	import { Apply } from '$lib/wailsjs/go/updater/Updater';
</script>

{#if $updateStore?.status === 'available'}
	<div class="toast" transition:fly={{ y: 40, duration: 200 }}>
		<div class="content">
			<p class="title">Update available</p>
			<p class="version">v{$updateStore.version} is ready to install</p>
		</div>
		<div class="actions">
			<Button content="Later" emphasis="low" size="sm" onclick={() => updateStore.set(null)} />
			<Button content="Update" emphasis="high" size="sm" onclick={() => Apply()} />
		</div>
	</div>
{/if}

<style>
	.toast {
		position: fixed;
		bottom: var(--space-md);
		right: var(--space-md);
		z-index: 6;
		display: flex;
		align-items: center;
		gap: var(--space-md);
		background: var(--gray-200);
		border: var(--border);
		border-radius: var(--br-xs);
		padding: var(--space-sm) var(--space-md);
		box-shadow: var(--shadow-lg, 0 4px 12px var(--shadow));
	}

	.content {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.title {
		font-size: var(--fs-sm);
		color: var(--gray-1000);
	}

	.version {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}

	.actions {
		display: flex;
		gap: var(--space-xs);
	}
</style>
