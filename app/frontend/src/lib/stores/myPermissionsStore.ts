import { writable, derived } from 'svelte/store';
import { GetMyPermissions } from '$lib/bindings/selectDb/internal/role/role';
import { must, tryCatch } from '$lib/utils/tryCatch';
import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
import {
	isAppActionAllowed,
	buildPermissionMap,
	resolve
} from '$lib/components/views/Settings/shared/permissions';
import type { Permission, PermissionMap } from '$lib/components/views/Settings/shared/permissions';

export const myPermissionsStore = writable<Permission[]>([]);

export async function loadMyPermissions(): Promise<void> {
	const raw = await must(tryCatch(GetMyPermissions));
	const permissions = (raw ?? []).map((p) => ({
		id: '',
		role_id: '',
		datasource_id: p.DatasourceID ?? null,
		schema_name: p.SchemaName ?? null,
		table_name: p.TableName ?? null,
		column_name: p.ColumnName ?? null,
		action: p.Action ?? '',
		effect: p.Effect as Permission['effect']
	})) satisfies Permission[];
	myPermissionsStore.set(permissions);
}

export function clearMyPermissions(): void {
	myPermissionsStore.set([]);
}

export const permissionActions = ['manage', 'select', 'see', 'insert', 'update', 'delete'];
export type PermissionActions = (typeof permissionActions)[number];

/** Reactive helper: check app-level and db-level permissions. */
export const myPermissions = derived(
	[myPermissionsStore, workspaceGraphStore],
	([$perms, $graph]) => {
		const isOwner = $graph?.is_owner ?? false;
		const permMap: PermissionMap = buildPermissionMap($perms);
		return {
			isAllowed: (action: string) => isAppActionAllowed($perms, action, isOwner),

			/**
			 * Whether this person administers every connection of the workspace, the
			 * question the backend asks before it adds one (`Actor.ManagesDatasources()`:
			 * owner, or workspace/datasources.manage).
			 */
			canManageDatasources: () =>
				isAppActionAllowed($perms, 'workspace/datasources.manage', isOwner),
			canAccessDatasource: (datasourceId: string, isProxified?: boolean) =>
				!isProxified ||
				isOwner ||
				permissionActions.some((a) => resolve(permMap, datasourceId, '*', '*', '*', a) === 'allow'),

			/**
			 * Whether this person administrates the connection, which is the same
			 * question the backend asks before it will change or revoke one
			 * (`Actor.ManagesDatasource(id)`: workspace-wide, or manage on it). Asking
			 * it here only decides what the UI offers: the server refuses either way.
			 */
			canManageDatasource: (datasourceId: string) =>
				isAppActionAllowed($perms, 'workspace/datasources.manage', isOwner) ||
				resolve(permMap, datasourceId, '*', '*', '*', 'manage') === 'allow'
		};
	}
);
