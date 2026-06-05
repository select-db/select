<script lang="ts">
	import { getActiveTab } from '$lib/components/Layout/layoutStore';

	const activeTab = $derived(getActiveTab());
</script>

{#if activeTab}
	<p>No file or database selected</p>
{:else}
	<div class="empty-state">
		<p>No file or database selected</p>
	</div>
{/if}

<style>
	.empty-state {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: var(--gray-800);
	}
</style>
