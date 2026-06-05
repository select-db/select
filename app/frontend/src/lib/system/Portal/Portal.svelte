<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	let {
		children,
		mount = document.body
	}: {
		children: import('svelte').Snippet;
		mount?: HTMLElement;
	} = $props();

	let portalEl: HTMLDivElement;

	onMount(() => {
		mount.appendChild(portalEl);
	});

	onDestroy(() => {
		// eslint-disable-next-line svelte/no-dom-manipulating
		portalEl.remove();
	});
</script>

<div bind:this={portalEl}>
	{@render children()}
</div>
