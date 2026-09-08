<script lang="ts">
	import type { ComponentProps } from 'svelte';
	import type SqlViewerMonaco from './SqlViewerMonaco.svelte';

	/**
	 * A read-only SQL preview, which monaco colourises.
	 *
	 * Monaco is 3.7MB and this component is reached from all over the shell --
	 * the history panel, the search modal, a table's field indicators, a chat
	 * tool card. Importing it here rather than there means every one of those
	 * paths used to put monaco in front of the app's first paint, whether or not
	 * any SQL was on screen. The split is at this boundary rather than at the
	 * call sites so there is one place to keep honest instead of five.
	 */
	let props: ComponentProps<typeof SqlViewerMonaco> = $props();

	const loading = import('./SqlViewerMonaco.svelte');
	let inner = $state<{ scrollToTop: () => void } | null>(null);

	export function scrollToTop() {
		inner?.scrollToTop();
	}
</script>

{#await loading then { default: Viewer }}
	<Viewer bind:this={inner} {...props} />
{/await}
