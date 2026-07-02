<script lang="ts">
	import { onDestroy } from 'svelte';
	import { isLeftbarOpened } from '$lib/components/Leftbar/store';

	import Resizer from './Resizer.svelte';
	import Content from './Content/Content.svelte';
	import Search from './Search/Search.svelte';
	import NewFileButton from './NewFileButton.svelte';
	import WorkspaceButton from './WorkspaceButton.svelte';

	const MIN_WIDTH = 0;
	const DEFAULT_WIDTH = 200;

	let closed = $state(false);
	let leftbarWidth: number = $state(
		parseInt(localStorage.getItem('leftbarWidth') || `${DEFAULT_WIDTH}`, 200)
	);
	let resizing: boolean = $state(false);
	let style = $derived(
		`${closed ? `min-width: ${MIN_WIDTH}px;max-width: ${MIN_WIDTH}px;` : `min-width: ${leftbarWidth}px;max-width: ${leftbarWidth}px;`}`
	);

	const unsubscribe = isLeftbarOpened.subscribe((isLeftbarOpened) => {
		closed = !isLeftbarOpened;
		if (isLeftbarOpened && leftbarWidth === MIN_WIDTH) leftbarWidth = DEFAULT_WIDTH;
	});

	onDestroy(() => unsubscribe());
</script>

<div class="wrapper" {style}>
	<Resizer bind:width={leftbarWidth} bind:resizing />

	<aside class="leftbar" class:resizing>
		<div class="actions-wrapper" style="--wails-draggable:drag">
			<WorkspaceButton />
			<div style="margin-left: auto;"></div>
			<Search />
			<NewFileButton />
		</div>
		<Content />
	</aside>
</div>

<style>
	.wrapper {
		position: relative;
		overflow: hidden;
		max-width: 420px;
	}

	.leftbar {
		display: flex;
		flex-direction: column;
		transition: width 0.1s ease-in;
		overflow: hidden;
		height: 100%;
	}

	.actions-wrapper {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding: 30px var(--space-sm-md) var(--space-sm) var(--space-sm);
		border-bottom: var(--border);
	}

	.leftbar.resizing {
		transition: none;
	}

	:global(.leftbar:hover .depth-spacer) {
		border-left-color: var(--gray-400);
	}
</style>
