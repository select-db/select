<script lang="ts">
	import { throttle } from '$lib/utils/throttle';
	import {
		updateIsRightbarOpened,
		RIGHTBAR_WIDTH_KEY,
		RIGHTBAR_MIN_WIDTH as MIN_WIDTH,
		RIGHTBAR_MAX_WIDTH as MAX_WIDTH
	} from './rightbarStore';

	// Owned by Rightbar, which seeds it from storage and pins the bar to it.
	export let width: number;
	export let resizing: boolean = false;

	const throttledResizeSidebar = throttle((e: MouseEvent) => {
		width = Math.max(MIN_WIDTH, Math.min(window.innerWidth - e.clientX, MAX_WIDTH));
		updateIsRightbarOpened(width > MIN_WIDTH);
		localStorage.setItem(RIGHTBAR_WIDTH_KEY, width.toString());
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
		border-left: var(--bw) var(--gray-800) solid;
	}

	#resizer:active {
		cursor: col-resize;
	}
</style>
