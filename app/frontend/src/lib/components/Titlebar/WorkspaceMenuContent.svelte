<script lang="ts">
	import Menu from '$lib/system/Menu/Menu.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { notify } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';
	import { tryCatch } from '$lib/utils/tryCatch';
	import {
		ListWorkspacesForCurrentUser,
		SwitchWorkspace,
		CreateWorkspaceAndReload
	} from '$lib/wailsjs/go/workspace/Workspace';
	import { Logout } from '$lib/wailsjs/go/system/System';
	import type { workspace } from '$lib/wailsjs/go/models';

	let workspaces = $state<workspace.WorkspaceWithCurrent[]>([]);
	let loading = $state(true);
	let searchQuery = $state('');

	async function load() {
		loading = true;
		const [list, err] = await tryCatch(ListWorkspacesForCurrentUser);
		loading = false;
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to load workspaces' });
			return;
		}
		workspaces = list ?? [];
	}

	async function switchTo(id: string) {
		const [, err] = await tryCatch(SwitchWorkspace, id);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to switch workspace' });
			return;
		}
		modalStore.set(null);
	}

	async function createAndSwitch() {
		const name = searchQuery.trim() || 'New workspace';
		const [, err] = await tryCatch(CreateWorkspaceAndReload, name);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to create workspace' });
			return;
		}
		modalStore.set(null);
		await Logout();
	}

	const menuOptions = $derived.by((): MenuOption<string>[] => {
		const options: MenuOption<string>[] = [];

		for (const ws of workspaces) {
			options.push({
				id: ws.id,
				label: ws.name,
				badge: ws.current ? 'current' : undefined,
				action: async () => {
					if (ws.current) return;
					await switchTo(ws.id);
				}
			});
		}

		const trimmedQuery = searchQuery.trim();
		const exactMatch =
			trimmedQuery && workspaces.some((w) => w.name.toLowerCase() === trimmedQuery.toLowerCase());
		const showCreateOption = trimmedQuery && !exactMatch;

		if (showCreateOption) {
			options.push({
				id: '__create__',
				label: `Create workspace '${trimmedQuery}'`,
				icon: 'plus',
				action: createAndSwitch
			});
		}

		return options;
	});

	$effect(() => {
		load();
	});
</script>

<Menu
	{loading}
	bind:searchQuery
	options={menuOptions}
	searchEnabled
	searchPlaceholder="Search or create workspace..."
	emptyMessage="No workspaces yet."
	noResultsMessage={`No workspaces match "${searchQuery}"`}
	onClose={() => modalStore.set(null)}
	maxHeight={360}
	width={360}
	noBorder
/>
