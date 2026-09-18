<script lang="ts">
	import type { ComponentProps } from 'svelte';
	import type CellValueModalMonaco from './CellValueModalMonaco.svelte';

	/**
	 * A cell's full value, in an editor.
	 *
	 * The third and last way monaco is reached: the results table imports this
	 * for a modal that only opens when a cell is expanded, which was enough to
	 * put 3.7MB in front of the app's first paint. Behind a dynamic import like
	 * the editor and the SQL preview, so no path into monaco is an eager one.
	 */
	let props: ComponentProps<typeof CellValueModalMonaco> = $props();

	const loading = import('./CellValueModalMonaco.svelte');
</script>

{#await loading then { default: Modal }}
	<Modal {...props} />
{/await}
