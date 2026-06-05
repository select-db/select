<script lang="ts">
	import { type Snippet } from 'svelte';

	type Props = Record<string, unknown>;

	function shallowDiff(prev: Props, next: Props) {
		const diffs: Record<string, { before: unknown; after: unknown }> = {};
		for (const key of Object.keys({ ...prev, ...next })) {
			if (prev[key] !== next[key]) {
				diffs[key] = { before: prev[key], after: next[key] };
			}
		}
		return diffs;
	}

	interface DebugRendersWrapperProps {
		debug: string;
		children: Snippet;
		[key: string]: unknown;
	}

	let { debug, children, ...restProps }: DebugRendersWrapperProps = $props();

	let el: HTMLElement | null = null;
	let prevProps: Props = {};
	let renderCount = 0;

	$effect(() => {
		console.log(import.meta.env.VITE_DEBUG_RERENDER);
		if (
			import.meta.env.VITE_DEBUG_RERENDER !== '*' &&
			import.meta.env.VITE_DEBUG_RERENDER !== debug
		)
			return;

		if (!el) return;

		renderCount++;

		console.log(`[${debug}][${renderCount}]:`, shallowDiff(prevProps, restProps));

		prevProps = { ...restProps };

		el.classList.add('rerender-blink');
		setTimeout(() => el!.classList.remove('rerender-blink'), 150);
	});
</script>

<div bind:this={el} class="debug-wrapper">
	{@render children()}
</div>

<style>
	.debug-wrapper {
		display: contents;
	}

	:global(.rerender-blink) {
		display: inherit !important;
		outline: 0.5px dashed var(--gray-1000);
		opacity: 0.8;
	}
</style>
