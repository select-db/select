<script lang="ts">
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import FilesButton from './FilesButton.svelte';
	import GitButton from './GitButton.svelte';
	import NetworkQuality from './NetworkQuality.svelte';
	import SearchButton from './SearchButton.svelte';
	import ThemeButton from './ThemeButton.svelte';
</script>

<div id="bottom-bar" class="wrapper">
	<NetworkQuality />
	<div class="divider"></div>
	{#if $workspaceGraphStore}
		<FilesButton />
		<div class="divider"></div>
		<GitButton />
		<div class="divider"></div>

		<div class="divider" style="margin-left: auto"></div>
		<SearchButton />
		<div class="divider"></div>
		<ThemeButton />
	{/if}
</div>

<style>
	.wrapper {
		display: flex;
		flex-wrap: nowrap;
		align-items: stretch;
		overflow: hidden;
		border-top: var(--border);
		background-color: var(--gray-0);
		height: 30px;

		z-index: 2;
	}

	:global(#bottom-bar.wrapper .wrapper) {
		padding-left: var(--space-sm);
		padding-right: var(--space-sm);
	}

	.divider {
		border-left: var(--border);
	}
</style>
