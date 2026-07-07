<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import GitLinkPanel from '$lib/components/views/Git/GitLinkPanel.svelte';
	import WorkspacePanel from '$lib/components/views/Settings/workspace/WorkspacePanel.svelte';
	import RolesPanel from '$lib/components/views/Settings/roles/RolesPanel.svelte';
	import GroupsPanel from '$lib/components/views/Settings/groups/GroupsPanel.svelte';
	import UsersPanel from '$lib/components/views/Settings/users/UsersPanel.svelte';
	import APIKeysPanel from '$lib/components/views/Settings/api-keys/APIKeysPanel.svelte';
	import { getActiveTab, updateSettingsTab } from '$lib/components/Layout/layoutStore';
	import { myPermissions } from '$lib/stores/myPermissionsStore';

	type SectionId = 'workspace' | 'users' | 'roles' | 'groups' | 'git' | 'api_keys';

	const sections: { id: SectionId; label: string }[] = [
		{ id: 'workspace', label: 'Workspace' },
		{ id: 'git', label: 'Git' },
		{ id: 'users', label: 'Users' },
		{ id: 'roles', label: 'Roles' },
		{ id: 'groups', label: 'Groups' },
		{ id: 'api_keys', label: 'API keys' }
	];

	const sectionAction: Record<SectionId, string> = {
		workspace: 'workspace/settings.write',
		git: 'workspace/settings.write',
		users: 'workspace/users.manage',
		roles: 'workspace/roles.manage',
		groups: 'workspace/groups.manage',
		api_keys: 'workspace/api-keys.manage'
	};

	const saved = getActiveTab()?.settings;
	let selectedSection = $state<SectionId>((saved?.section as SectionId) ?? 'workspace');

	function canAccess(id: SectionId): boolean {
		const action = sectionAction[id];
		return !action || $myPermissions.isAllowed(action);
	}

	function selectSection(id: SectionId) {
		selectedSection = id;
		updateSettingsTab({ section: id, roles: { selectedRoleId: null, selectedRoleName: null } });
	}
</script>

<div class="settings-view">
	<nav class="settings-nav">
		{#each sections as section (section.id)}
			<Button
				content={section.label}
				emphasis={selectedSection === section.id ? 'high' : 'low'}
				active={selectedSection === section.id}
				onclick={() => selectSection(section.id)}
				classes="nav-item"
				noBounce
			/>
		{/each}
	</nav>
	<main class="settings-content">
		{#if !canAccess(selectedSection)}
			<div class="build-in-progress">
				<Icon icon="cross" size={32} />
				<p class="title">Access restricted</p>
				<p class="hint">You don't have permission to view this section.</p>
			</div>
		{:else if selectedSection === 'workspace'}
			<WorkspacePanel />
		{:else if selectedSection === 'roles'}
			<RolesPanel />
		{:else if selectedSection === 'groups'}
			<GroupsPanel />
		{:else if selectedSection === 'users'}
			<UsersPanel />
		{:else if selectedSection === 'api_keys'}
			<APIKeysPanel />
		{:else if selectedSection === 'git'}
			<GitLinkPanel showUnsync />
		{:else}
			<div class="build-in-progress">
				<Icon icon="cog" size={32} />
				<p class="title">Build in progress</p>
				<p class="hint">This section is not available yet.</p>
			</div>
		{/if}
	</main>
</div>

<style>
	.settings-view {
		display: flex;
		height: 100%;
		overflow: hidden;
	}

	.settings-nav {
		display: flex;
		gap: var(--space-xs);
		padding: var(--space-sm);
		flex-direction: column;
		width: 120px;
		min-width: 120px;
		background-color: var(--gray-0);
	}

	.settings-nav :global(.nav-item) {
		justify-content: flex-start;
		width: 100%;
		height: 26px;
		font-size: var(--fs-sm);
	}

	.settings-content {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		background-color: var(--gray-0);
	}

	.settings-content :global(.panel-scroll) {
		flex: 1;
		overflow: auto;
		padding: var(--space-sm-md) 0;
	}

	.build-in-progress {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: start;
		padding-top: var(--space-lg);
		gap: var(--space-sm);
		height: 100%;
		min-height: 280px;
		color: var(--gray-800);
	}

	.build-in-progress .title {
		font-size: var(--fs-sm);
		font-weight: 600;
	}

	.build-in-progress .hint {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}
</style>
