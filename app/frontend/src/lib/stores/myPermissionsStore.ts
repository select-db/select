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
		db_instance_id: p.DbInstanceID ?? null,
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

export const permissionActions = ['manage', 'select', 'see', 'insert', 'update', 'delete', 'ddl'];
export type PermissionActions = (typeof permissionActions)[number];

/** Reactive helper: check app-level and db-level permissions. */
export const myPermissions = derived(
	[myPermissionsStore, workspaceGraphStore],
	([$perms, $graph]) => {
		const isOwner = $graph?.is_owner ?? false;
		const permMap: PermissionMap = buildPermissionMap($perms);
		return {
			isAllowed: (action: string) => isAppActionAllowed($perms, action, isOwner),
			canAccessDb: (dbId: string, isProxified?: boolean) =>
				!isProxified ||
				isOwner ||
				permissionActions.some((a) => resolve(permMap, dbId, '*', '*', '*', a) === 'allow')
		};
	}
);
