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
	} from '$lib/bindings/selectDb/internal/workspace/workspace';
	import { Logout } from '$lib/bindings/selectDb/internal/system/system';
	import type * as workspace from '$lib/bindings/selectDb/internal/workspace/models';
	import Avatar from '$lib/system/Avatar/Avatar.svelte';
	import { logoSrc } from '$lib/utils/workspaceLogo';

	let workspaces = $state<workspace.WorkspaceWithCurrent[]>([]);
	let loading = $state(true);

	const currentId = $derived(
		workspaces.find((w) => w.current)?.id ?? $workspaceGraphStore?.id ?? ''
	);
	const options = $derived<SelectOption[]>(workspaces.map((w) => ({ value: w.id, label: w.name })));

	// SelectOption carries only a value and a label, so the logo is looked up by
	// id from the row the option came from. Keeps the shared Select untouched.
	const logoById = $derived(new Map(workspaces.map((w) => [w.id, logoSrc(w.logo)])));

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
		// Re-list whenever the workspace graph changes, so a rename or a logo
		// saved in Settings shows up here without remounting the button.
		void $workspaceGraphStore?.name;
		void $workspaceGraphStore?.logo;
		load();
	});
</script>

{#snippet workspaceOption(option: SelectOption | null)}
	<span class="workspace-option">
		<Avatar
			src={option ? (logoById.get(option.value as string) ?? null) : null}
			name={option?.label}
			size={16}
			shape="rounded"
		/>
		<span class="workspace-name">{option?.label ?? 'Workspace'}</span>
	</span>
{/snippet}

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
	optionDisplay={workspaceOption}
/>

<style>
	.workspace-option {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		min-width: 0;
	}

	.workspace-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
