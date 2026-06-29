<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import GitLinkPanel from '$lib/components/views/Git/GitLinkPanel.svelte';
	import WorkspacePanel from '$lib/components/views/Settings/WorkspacePanel.svelte';
	import RolesPanel from '$lib/components/views/Settings/RolesPanel.svelte';
	import UsersPanel from '$lib/components/views/Settings/UsersPanel.svelte';
	import APIKeysPanel from '$lib/components/views/Settings/APIKeysPanel.svelte';
	import UserSettingsEditor from '$lib/components/views/Settings/UserSettingsEditor.svelte';
	import { getActiveTab, updateSettingsTab } from '$lib/components/Layout/layoutStore';
	import { myPermissions } from '$lib/stores/myPermissionsStore';
	import type { Icons } from '$lib/system/Icon/types';

	type SectionId = 'workspace' | 'users' | 'roles' | 'git' | 'api_keys' | 'theme' | 'config';

	const sections: { id: SectionId; label: string; icon: Icons }[] = [
		{ id: 'workspace', label: 'Workspace', icon: 'folder' },
		{ id: 'git', label: 'Git', icon: 'github-branch' },
		{ id: 'users', label: 'Users', icon: 'users' },
		{ id: 'roles', label: 'Roles', icon: 'roles' },
		{ id: 'api_keys', label: 'API keys', icon: 'key' },
		{ id: 'theme', label: 'Theme', icon: 'theme' },
		{ id: 'config', label: 'Config', icon: 'cog' }
	];

	// Personal sections (theme, config) have no permission gate: they are the
	// user's own settings and apply across all workspaces.
	const sectionAction: Partial<Record<SectionId, string>> = {
		workspace: 'workspace/settings.write',
		git: 'workspace/settings.write',
		users: 'workspace/users.manage',
		roles: 'workspace/roles.manage',
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
				leftIcon={section.icon}
				iconSize={16}
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
		{:else if selectedSection === 'users'}
			<UsersPanel />
		{:else if selectedSection === 'api_keys'}
			<APIKeysPanel />
		{:else if selectedSection === 'git'}
			<GitLinkPanel showUnsync />
		{:else if selectedSection === 'theme'}
			<UserSettingsEditor kind="theme" />
		{:else if selectedSection === 'config'}
			<UserSettingsEditor kind="config" />
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
		height: 28px;
		font-size: var(--fs-sm);
		gap: var(--space-xs);
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
