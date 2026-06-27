<script lang="ts">
	import { onDestroy } from 'svelte';
	import { isLeftbarOpened } from '$lib/components/Leftbar/store';

	import Resizer from './Resizer.svelte';
	import Content from './Content/Content.svelte';

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
		<Content />
	</aside>
</div>

<style>
	.wrapper {
		position: relative;
		overflow: hidden;
	}

	.leftbar {
		display: flex;
		flex-direction: column;
		transition: width 0.1s ease-in;
		overflow: hidden;
		height: 100%;
	}

	.leftbar.resizing {
		transition: none;
	}

	:global(.leftbar:hover .depth-spacer) {
		border-left-color: var(--gray-600);
	}
</style>
