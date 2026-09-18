<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import type { Icons } from '$lib/system/Icon/types';

	type ModalHeaderProps = {
		icon?: Icons;
		title: string;
	};

	let { icon, title }: ModalHeaderProps = $props();
</script>

<div class="header">
	{#if icon}
		<Icon {icon} size={16}></Icon>
	{/if}
	<p>{title}</p>
</div>

<style>
	.header {
		display: flex;
		gap: var(--space-sm);
		background-color: var(--gray-200);
		padding: var(--space-sm);
		border-bottom: var(--border);
		z-index: 1;
	}
</style>
