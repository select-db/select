<script lang="ts">
	import { throttle } from '$lib/utils/throttle';
	import { updateIsRightbarOpened } from './rightbarStore';

	const MIN_WIDTH = 0;
	const DEFAULT_WIDTH = 500;

	export let width: number = parseInt(
		localStorage.getItem('rightbarWidth') || `${DEFAULT_WIDTH}`,
		200
	);
	export let resizing: boolean = false;

	const throttledResizeSidebar = throttle((e: MouseEvent) => {
		width = Math.max(MIN_WIDTH, window.innerWidth - e.clientX);
		updateIsRightbarOpened(width > MIN_WIDTH);
		localStorage.setItem('rightbarWidth', width.toString());
	}, 15);

	function startResizing(e: MouseEvent) {
		resizing = true;
		e.preventDefault();
		document.addEventListener('mousemove', throttledResizeSidebar);
		document.addEventListener('mouseup', stopResizing);
	}

	function stopResizing() {
		resizing = false;
		document.removeEventListener('mousemove', throttledResizeSidebar);
		document.removeEventListener('mouseup', stopResizing);
	}
</script>

<div
	id="resizer"
	role="presentation"
	class={resizing ? 'resizing' : ''}
	onmousedown={startResizing}
></div>

<style>
	#resizer {
		position: absolute;
		top: 64px;
		left: 1px;
		bottom: var(--space-sm-md);

		width: 10px;
		border-left: var(--bw) transparent solid;
		margin-left: -1px;

		cursor: col-resize;

		transition: border-left 0.2s 0.2s ease-in;
	}

	#resizer:hover,
	#resizer.resizing {
		border-left: var(--bw) var(--white-glow) solid;
	}

	#resizer:active {
		cursor: col-resize;
	}
</style>
