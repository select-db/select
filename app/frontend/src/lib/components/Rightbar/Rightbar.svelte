<script lang="ts">
	import { onDestroy } from 'svelte';

	import Search from '$lib/components/views/Search/Search.svelte';
	import History from '$lib/components/views/History/History.svelte';

	import Resizer from './Resizer.svelte';
	import {
		isRightbarOpened,
		rightPanelTab,
		RIGHTBAR_WIDTH_KEY,
		RIGHTBAR_MIN_WIDTH as MIN_WIDTH,
		RIGHTBAR_MAX_WIDTH as MAX_WIDTH,
		RIGHTBAR_DEFAULT_WIDTH as DEFAULT_WIDTH
	} from './rightbarStore';
	import { readStoredWidth } from '$lib/utils/storedWidth';

	let closed = $state(false);
	let rightbarWidth: number = $state(
		Math.min(readStoredWidth(RIGHTBAR_WIDTH_KEY, DEFAULT_WIDTH), MAX_WIDTH)
	);
	let resizing: boolean = $state(false);
	let width = $derived(closed ? MIN_WIDTH : rightbarWidth);
	// Pin the container to the same width as the bar: with the width only on the
	// inner <aside>, the container still sized to its content.
	let containerStyle = $derived(`min-width: ${width}px; max-width: ${width}px;`);
	let style = $derived(`width: ${width}px;${closed ? ' padding-left: 0;' : ''}`);

	const unsubscribe = isRightbarOpened.subscribe((v) => {
		closed = !v;
		if (v && rightbarWidth === MIN_WIDTH) rightbarWidth = DEFAULT_WIDTH;
	});

	onDestroy(() => unsubscribe());
</script>

<div id="rightbarContainer" style={containerStyle}>
	<Resizer bind:width={rightbarWidth} bind:resizing />

	<aside id="rightbar" {style} class:resizing>
		{#if $rightPanelTab === 'search'}
			<Search />
		{:else if $rightPanelTab === 'history'}
			<History />
		{/if}
	</aside>
</div>

<style>
	#rightbarContainer {
		position: relative;
		padding-top: var(--space-sm-md);
		/* Width lives on the inner #rightbar, so pin this flex item too —
		   otherwise wide tab content can shrink/push the bar. */
		flex-shrink: 0;
	}

	#rightbar {
		height: 100%;
		display: flex;
		flex-direction: column;
		transition: width 0.1s ease-in;
		overflow: hidden;
	}

	#rightbar.resizing {
		transition: none;
	}
</style>
