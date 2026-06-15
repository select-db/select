<script lang="ts">
	import { onDestroy, onMount } from 'svelte';

	import Modal from '$lib/system/Modal/Modal.svelte';
	import Notifications from '$lib/system/Notifications/Notifications.svelte';
	import Tooltips from '$lib/system/Tooltip/Tooltips.svelte';

	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import {
		loadGitStatus,
		gitWorkspaceStatusStore,
		gitFileStatusStore
	} from '$lib/components/views/Git/gitStore';
	import { StartFileWatcher, StartDatabaseWatcher } from '$lib/wailsjs/go/system/System';

	import Leftbar from '$lib/components/Leftbar/Leftbar.svelte';
	import Titlebar from '$lib/components/Titlebar/Titlebar.svelte';
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
	import { zoomStore } from '$lib/stores/zoomStore';
	import KeybindingsManager from '$lib/components/KeybindingsManager.svelte';

	import { setupSessionWall, teardownSessionWall, sessionCheckingStore } from './sessionWall';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { initLayoutPersistence } from '$lib/components/Layout/layoutPersistence';
	import { EventsOn } from '$lib/wailsjs/runtime/runtime';
	import { CheckVersion } from '$lib/wailsjs/go/updater/Updater';
	import { updateStore } from '$lib/stores/updateStore';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';
	import UpdateOverlay from '$lib/components/UpdateOverlay/UpdateOverlay.svelte';
	import UpdateToast from '$lib/components/UpdateOverlay/UpdateToast.svelte';

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
		setupSessionWall();
		initTheme();
		teardownLayoutPersistence = initLayoutPersistence();
		CheckVersion();
	});

	onDestroy(() => {
		teardownSessionWall();
		teardownLayoutPersistence?.();
	});

	let watchedWorkspaceId: string | undefined;
	$: if ($workspaceGraphStore && $workspaceGraphStore.id !== watchedWorkspaceId) {
		watchedWorkspaceId = $workspaceGraphStore.id;
		gitWorkspaceStatusStore.set(null);
		gitFileStatusStore.set(null);
		loadGitStatus();
		StartFileWatcher($workspaceGraphStore.id);
		StartDatabaseWatcher();
	}

	$: setContext('leftPanelVisible', $isLeftbarOpened);
	$: setContext('rightPanelVisible', $isRightbarOpened);
	$: document.documentElement.style.zoom = String($zoomStore);
</script>

<div class="wrapper">
	<Titlebar />

	<div class="layout">
		{#if $sessionCheckingStore}
			<div class="session-loader"><Loader size={24} /></div>
		{:else if $workspaceGraphStore}
			{#key `${$themeVersionStore}-${$configVersionStore}-${$lintVersionStore}`}
				<Leftbar />
				<main class:no-left-border={!$isLeftbarOpened} class:no-right-border={!$isRightbarOpened}>
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
		background-color: var(--gray-0);
		border: var(--border);
		border-radius: var(--br-sm);
		flex-grow: 1;
		overflow: hidden;

		z-index: 2;
	}

	main.no-left-border {
		border-top-left-radius: 0;
		border-bottom-left-radius: 0;
		border-left: none;
	}

	main.no-right-border {
		border-top-right-radius: 0;
		border-bottom-right-radius: 0;
		border-right: none;
	}

	.session-loader {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
	}
</style>
