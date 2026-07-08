<script lang="ts">
	import Select from '$lib/system/Select/Select.svelte';
	import type { SelectOption } from '$lib/system/Select/Select.types';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
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

	const currentId = $derived(
		workspaces.find((w) => w.current)?.id ?? $workspaceGraphStore?.id ?? ''
	);
	const options = $derived<SelectOption[]>(workspaces.map((w) => ({ value: w.id, label: w.name })));

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
		if (!id || id === currentId) return;
		const [, err] = await tryCatch(SwitchWorkspace, id);
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to switch workspace' });
		}
	}

	async function createAndSwitch(name: string) {
		const [, err] = await tryCatch(CreateWorkspaceAndReload, name.trim() || 'New workspace');
		if (err) {
			notify({ type: AlertType.Error, message: err?.message ?? 'Failed to create workspace' });
			return;
		}
		await Logout();
	}

	$effect(() => {
		load();
	});
</script>

<Select
	value={currentId}
	{options}
	onchange={(v) => switchTo(v as string)}
	isLoading={loading}
	placeholder="Workspace"
	searchEnabled
	searchPlaceholder="Search or create workspace..."
	createOptionLabel={(q) => `Create workspace '${q}'`}
	onCreate={createAndSwitch}
	menuWidth={300}
	emphasis="low"
/>
