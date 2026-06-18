<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Search from './Search/Search.svelte';
	import UserAvatar from './UserAvatar.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { useNativeWindowControls } from '$lib/utils/platform';
	import {
		WindowClose,
		WindowMinimise,
		WindowToggleFullscreen
	} from '$lib/wailsjs/go/system/System';

	function closeApp() {
		WindowClose();
	}
	function minimize() {
		WindowMinimise();
	}
	function maximize() {
		WindowToggleFullscreen();
	}
</script>

<div
	class="titlebar"
	class:native-controls={useNativeWindowControls}
	style="--wails-draggable:drag"
>
	<div class="wrapper start">
		{#if useNativeWindowControls}
			<div class="native-controls-spacer" aria-hidden="true"></div>
		{:else}
			<div class="window-actions left" style="--wails-draggable:false">
				<button class="window-action close" onclick={closeApp}>
					<Icon icon="cross" size={8} stroke="black" fill="black" strokeWidth={2} />
				</button>

				<button class="window-action minimize" onclick={minimize}>
					<Icon icon="minus" size={8} stroke="black" fill="black" strokeWidth={2} />
				</button>

				<button class="window-action maximize" onclick={maximize}>
					<Icon icon="arrow-diagonal" size={8} stroke="black" fill="black" strokeWidth={2} />
				</button>
			</div>
		{/if}
	</div>

	<div class="wrapper center">
		{#if $workspaceGraphStore}
			<Search />
		{/if}
	</div>

	<div class="wrapper end">
		{#if $workspaceGraphStore}
			<UserAvatar />
		{/if}
	</div>
</div>

<style>
	.titlebar {
		position: relative;
		display: flex;
		align-items: stretch;
		justify-content: space-between;
		height: 36px;
		overflow: hidden;
		padding-right: var(--space-xs-sm);
	}

	.window-actions {
		display: flex;
		align-items: center;
	}

	/* Reserve space for native traffic lights on macOS so content doesn't overlap */
	.native-controls-spacer {
		min-width: 70px;
	}

	.window-actions.left {
		gap: var(--space-sm);
		padding: var(--space-sm) var(--space-xs) var(--space-sm) var(--space-sm-md);
	}

	.wrapper {
		flex: 1;
		display: flex;
		overflow: hidden;
		align-items: center;
	}

	.wrapper.start {
		justify-content: start;
	}

	.wrapper.center {
		justify-content: center;
	}

	.wrapper.end {
		justify-content: end;
	}

	.window-action {
		width: 12px;
		height: 12px;
		border-radius: 50%;
		border: none;

		display: flex;
		align-items: center;
		justify-content: center;

		transition: background-color 0.1s ease-in;
	}

	:global(.window-actions .window-action svg) {
		visibility: hidden;
	}
	:global(.window-actions:hover .window-action svg) {
		visibility: visible;
	}

	.close,
	.minimize,
	.maximize {
		background-color: var(--gray-800);
	}

	.window-actions:hover .close {
		background-color: #ff5f57;
	}

	.window-actions:hover .minimize {
		background-color: #ffbd2e;
	}

	.window-actions:hover .maximize {
		background-color: #27c93f;
	}

	.window-action:hover {
		opacity: 0.8;
	}

	:global(.titlebar button) {
		height: 24px;
	}
	:global(.titlebar button svg),
	:global(.titlebar button p) {
		stroke: var(--gray-700) !important;
		color: var(--gray-700) !important;
	}
	:global(.titlebar button:hover svg),
	:global(.titlebar button:hover p) {
		stroke: var(--gray-1000) !important;
		color: var(--gray-1000) !important;
	}
</style>
