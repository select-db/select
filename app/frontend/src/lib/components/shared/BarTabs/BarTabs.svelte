<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import type { Icons } from '$lib/system/Icon/types';

	let {
		tabs,
		activeTab = $bindable(),
		tabBadges
	}: {
		tabs: Icons[];
		activeTab: Icons;
		tabBadges?: Partial<Record<Icons, string | number>>;
	} = $props();
</script>

<div class="nav-wrapper">
	{#each tabs as tab (tab)}
		{@const isActive = activeTab === tab}
		<Button
			leftIcon={tab}
			iconSize={18}
			size="sm"
			onclick={() => {
				activeTab = tab;
			}}
			emphasis={isActive ? 'high' : 'low'}
			active={isActive}
			badge={tabBadges?.[tab]}
		/>
	{/each}
</div>

<style>
	.nav-wrapper {
		display: flex;
		gap: var(--space-xs-sm);
		padding: var(--space-sm-md) var(--space-sm-md) var(--space-sm) var(--space-sm-md);
	}

	:global(.nav-wrapper button) {
		border: var(--bw) transparent solid !important;
		transition: border-color 0.1s ease-out 0.03s;
	}
	:global(.nav-wrapper button:not(.active):hover) {
		background-color: var(--gray-400) !important;
		border-color: var(--border-color) !important;
	}
	:global(.nav-wrapper button.active) {
		background-color: var(--gray-0) !important;
		border-color: var(--border-color) !important;
	}
</style>
