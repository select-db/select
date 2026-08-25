<script lang="ts">
	import { onDestroy, onMount, type Snippet } from 'svelte';

	import Modal from '$lib/system/Modal/Modal.svelte';
	import Notifications from '$lib/system/Notifications/Notifications.svelte';
	import Tooltips from '$lib/system/Tooltip/Tooltips.svelte';

	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import {
		loadGitStatus,
		gitWorkspaceStatusStore,
		gitFileStatusStore
	} from '$lib/components/views/Git/gitStore';
	import {
		StartFileWatcher,
		StartDatabaseWatcher
	} from '$lib/bindings/selectDb/internal/system/system';

	import Leftbar from '$lib/components/Leftbar/Leftbar.svelte';
	import PageLogin from '$lib/components/PageLogin/PageLogin.svelte';
	import Rightbar from '$lib/components/Rightbar/Rightbar.svelte';
	import Bottombar from '$lib/components/Bottombar/Bottombar.svelte';
	import EditorLayout from '$lib/components/Layout/Layout.svelte';
	import { layoutStore } from '$lib/components/Layout/layoutStore';
	import { isLeftbarOpened } from '$lib/components/Leftbar/store';
	import { isRightbarOpened } from '$lib/components/Rightbar/rightbarStore';
	import { themeVersionStore, initTheme } from '$lib/stores/themeStore';
	import { configVersionStore } from '$lib/stores/keybindingsStore';
	import { lintVersionStore } from '$lib/stores/lintStore';
	import '$lib/utils/query/queryStream.svelte';
	import { setContext } from '$lib/stores/keybindingsContextStore';
	import KeybindingsManager from '$lib/components/KeybindingsManager.svelte';
	import { initZoom } from '$lib/wails/zoom';

	import { setupSessionWall, teardownSessionWall, sessionCheckingStore } from './sessionWall';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { initLayoutPersistence } from '$lib/components/Layout/layoutPersistence';
	import { EventsOn } from '$lib/wails/events';
	import { CheckVersion } from '$lib/bindings/selectDb/internal/updater/updater';
	import { updateStore } from '$lib/stores/updateStore';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';
	import UpdateOverlay from '$lib/components/UpdateOverlay/UpdateOverlay.svelte';
	import UpdateToast from '$lib/components/UpdateOverlay/UpdateToast.svelte';

	// SvelteKit hands every layout its page's content. Ours is deliberately
	// empty — see +page.svelte — but it is rendered where a page would go, so
	// nothing is silently dropped.
	let { children }: { children?: Snippet } = $props();

	let teardownLayoutPersistence: (() => void) | undefined;

	EventsOn(
		'update',
		(e: { status: string; progress: number; version: string; message: string }) => {
			if (e.status === 'error') {
				updateStore.set(null);
				notifyError(`Update failed: ${e.message}`);
			} else {
				updateStore.set({
					status: e.status as 'available' | 'downloading' | 'installing',
					progress: e.progress,
					version: e.version,
					message: e.message
				});
			}
		}
	);

	onMount(() => {
		void initZoom();
		setupSessionWall();
		initTheme();
		teardownLayoutPersistence = initLayoutPersistence();
		CheckVersion();
	});

	onDestroy(() => {
		teardownSessionWall();
		teardownLayoutPersistence?.();
	});

	// Runes, because this component takes props now and the two modes cannot be
	// mixed. These ran on init before and run after mount now, which neither the
	// watchers nor the keybinding context care about.
	let watchedWorkspaceId: string | undefined;
	$effect(() => {
		const workspace = $workspaceGraphStore;
		if (!workspace || workspace.id === watchedWorkspaceId) return;

		watchedWorkspaceId = workspace.id;
		gitWorkspaceStatusStore.set(null);
		gitFileStatusStore.set(null);
		loadGitStatus();
		StartFileWatcher(workspace.id);
		StartDatabaseWatcher();
	});

	$effect(() => setContext('leftPanelVisible', $isLeftbarOpened));
	$effect(() => setContext('rightPanelVisible', $isRightbarOpened));
</script>

<div class="wrapper">
	<div class="layout">
		{#if $sessionCheckingStore}
			<div class="session-loader"><Loader size={24} /></div>
		{:else if $workspaceGraphStore}
			{#key `${$themeVersionStore}-${$configVersionStore}-${$lintVersionStore}`}
				<Leftbar />
				<main class:left-bar-closed={!$isLeftbarOpened} class:right-bar-closed={!$isRightbarOpened}>
					{@render children?.()}
					<div class="drag-spacer" style="--wails-draggable:drag"></div>
					<EditorLayout node={$layoutStore.root} />
				</main>
				<Rightbar />
			{/key}
			<Tooltips />

			<KeybindingsManager />
		{:else}
			<PageLogin />
		{/if}
	</div>

	<Notifications />
	<Modal />

	<UpdateOverlay />
	<UpdateToast />

	<Bottombar />
</div>

<style>
	@import './app.css';

	.wrapper {
		display: flex;
		flex-direction: column;
		overflow: hidden;

		height: 100vh;
	}

	.layout {
		display: flex;
		flex: 1;
		overflow: hidden;
	}

	main {
		display: flex;
		flex-direction: column;
		/* flex-basis:0 + min-width:0 → main's width is purely the leftover space
		   between the side bars, never influenced by (over)wide tab content.
		   Combined with overflow:hidden, content clips/scrolls inside main. */
		flex: 1 1 0;
		min-width: 0;
		overflow: hidden;
		z-index: 2;

		box-shadow: var(--shadow-main);
	}

	.drag-spacer {
		flex-shrink: 0;
		height: var(--space-sm);
	}
	main.left-bar-closed {
		padding-left: var(--space-sm);
	}
	main.right-bar-closed {
		padding-right: var(--space-sm);
	}

	.session-loader {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
	}
</style>
