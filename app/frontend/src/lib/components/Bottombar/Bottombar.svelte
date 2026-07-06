<script lang="ts">
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import GitBranch from './GitBranch.svelte';
	import FilesButton from './FilesButton.svelte';
	import GitButton from './GitButton.svelte';
	import NetworkQuality from './NetworkQuality.svelte';
	import SearchButton from './SearchButton.svelte';
	import HistoryButton from './HistoryButton.svelte';
	import ThemeButton from './ThemeButton.svelte';
	import UserAvatar from './UserAvatar.svelte';
</script>

<div id="bottom-bar" class="wrapper">
	<NetworkQuality />
	{#if $workspaceGraphStore}
		<FilesButton />
		<div class="divider"></div>
		<GitButton />
		<div class="divider"></div>
		<UserAvatar />
		<div class="divider"></div>
		<GitBranch />

		<div style="margin-left: auto"></div>
		<HistoryButton />
		<div class="divider"></div>
		<SearchButton />
		<div class="divider"></div>
		<ThemeButton />
	{/if}
</div>

<style>
	.wrapper {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		overflow: hidden;
		padding-right: var(--space-sm);

		height: 42px;

		z-index: 2;
	}

	.divider {
		width: var(--space-xs-sm);
	}

	:global(#bottom-bar.wrapper .wrapper) {
		padding-left: var(--space-sm);
		padding-right: var(--space-sm);
	}

	:global(#bottom-bar button) {
		height: 24px;
	}
</style>
