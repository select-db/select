<script lang="ts">
	import { onDestroy } from 'svelte';

	import Search from '$lib/components/views/Search/Search.svelte';
	import History from '$lib/components/views/History/History.svelte';

	import Resizer from './Resizer.svelte';
	import { isRightbarOpened, rightPanelTab } from './rightbarStore';

	const MIN_WIDTH = 0;
	const DEFAULT_WIDTH = 300;

	let closed = $state(false);
	let rightbarWidth: number = $state(
		parseInt(localStorage.getItem('rightbarWidth') || `${DEFAULT_WIDTH}`, 200)
	);
	let resizing: boolean = $state(false);
	let style = $derived(
		`${closed ? `width: ${MIN_WIDTH}px; padding-left: 0;` : `width: ${rightbarWidth}px;`}`
	);

	const unsubscribe = isRightbarOpened.subscribe((v) => {
		closed = !v;
		if (v && rightbarWidth === MIN_WIDTH) rightbarWidth = DEFAULT_WIDTH;
	});

	onDestroy(() => unsubscribe());
</script>

<div id="rightbarContainer">
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
