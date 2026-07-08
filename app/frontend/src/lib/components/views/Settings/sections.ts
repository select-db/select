import type { Icons } from '$lib/system/Icon/types';

export type SettingsSectionId =
	| 'workspace'
	| 'git'
	| 'users'
	| 'roles'
	| 'groups'
	| 'api_keys'
	| 'theme'
	| 'config';

export type SettingsSection = {
	id: SettingsSectionId;
	label: string;
	icon: Icons;
	/**
	 * Permission required to view the section. Undefined = always accessible
	 * (personal sections like theme/config apply across all workspaces).
	 */
	action?: string;
};

/** Single source of truth for Settings sections (nav, tab labels, resource menu). */
export const settingsSections: SettingsSection[] = [
	{ id: 'workspace', label: 'Workspace', icon: 'folder', action: 'workspace/settings.write' },
	{ id: 'git', label: 'Git', icon: 'github-branch', action: 'workspace/settings.write' },
	{ id: 'users', label: 'Users', icon: 'users', action: 'workspace/users.manage' },
	{ id: 'roles', label: 'Roles', icon: 'roles', action: 'workspace/roles.manage' },
	{ id: 'groups', label: 'Groups', icon: 'group', action: 'workspace/groups.manage' },
	{ id: 'api_keys', label: 'API keys', icon: 'key', action: 'workspace/api-keys.manage' },
	{ id: 'theme', label: 'Theme', icon: 'theme' },
	{ id: 'config', label: 'Config', icon: 'cog' }
];

export const settingsSectionLabels: Record<string, string> = Object.fromEntries(
	settingsSections.map((s) => [s.id, s.label])
);
