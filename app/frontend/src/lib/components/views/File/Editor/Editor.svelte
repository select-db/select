<script lang="ts">
	import type { ComponentProps } from 'svelte';
	import type EditorMonaco from './EditorMonaco.svelte';

	/**
	 * The SQL editor, which is monaco.
	 *
	 * Monaco is 3.7MB of the app and none of it is on screen until a file is
	 * open, but it used to be parsed before the app could paint at all: the root
	 * layout reaches every view eagerly, so opening the app paid for the editor
	 * whether or not it was going to be used. Loading it here keeps that cost
	 * where it belongs -- the first file opened -- and every caller stays as it
	 * was, including the two that hold this by `bind:this`.
	 */
	let props: ComponentProps<typeof EditorMonaco> = $props();

	const loading = import('./EditorMonaco.svelte');
	let inner = $state<{ format: () => void; focus: () => void } | null>(null);

	/**
	 * Focus asked for before monaco arrived, replayed once it has. A tab opening
	 * focuses its editor in the same tick it is created, which is now before
	 * there is an editor to focus; dropping it would put the caret nowhere.
	 */
	let focusWanted = false;

	$effect(() => {
		if (!inner || !focusWanted) return;
		focusWanted = false;
		inner.focus();
	});

	export function format() {
		inner?.format();
	}

	export function focus() {
		if (inner) inner.focus();
		else focusWanted = true;
	}
</script>

{#await loading then { default: Monaco }}
	<Monaco bind:this={inner} {...props} />
{/await}
