// Resolution walks most-to-least specific on (db, schema, table, col, action).
// '*' = wildcard, '' = app-level (no db scope).

export type Effect = 'allow' | 'deny' | 'none';

export type Permission = {
	id: string;
	role_id: string;
	db_instance_id: string | null;
	schema_name: string | null;
	table_name: string | null;
	column_name: string | null;
	action: string;
	effect: Effect;
};

export type PermissionEntry = { id: string; effect: Effect };
export type PermissionMap = Map<string, PermissionEntry>;

export const permissionKey = (
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string
): string => {
	return `${db}|${schema}|${table}|${col}|${action}`;
};

export function buildPermissionMap(permissions: Permission[]): PermissionMap {
	const map = new Map<string, PermissionEntry>();
	for (const p of permissions) {
		const key = permissionKey(
			p.db_instance_id ?? '',
			p.schema_name ?? '',
			p.table_name ?? '',
			p.column_name ?? '',
			p.action
		);
		map.set(key, { id: p.id, effect: p.effect });
	}
	return map;
}

export function resolve(
	map: PermissionMap,
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string
): Effect {
	const candidates = [
		permissionKey(db, schema, table, col, action),
		permissionKey(db, schema, table, '*', action),
		permissionKey(db, schema, '*', '*', action),
		permissionKey(db, '*', '*', '*', action)
	];
	for (const k of candidates) {
		const entry = map.get(k);
		if (entry) return entry.effect;
	}
	return 'none';
}

export function isAllowed(
	map: PermissionMap,
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string
): boolean {
	return resolve(map, db, schema, table, col, action) === 'allow';
}

export function isAppActionAllowed(
	permissions: Permission[],
	action: string,
	isOwner: boolean
): boolean {
	if (isOwner) return true;
	for (const p of permissions) {
		if (p.db_instance_id !== null) continue; // db-scoped rule, skip
		if (p.action === action && p.effect === 'allow') return true;
	}
	return false;
}

export type PermissionChange =
	| {
			type: 'add';
			db: string;
			schema: string;
			table: string;
			col: string;
			action: string;
			effect: 'allow' | 'deny';
	  }
	| { type: 'remove'; id: string };

function isDescendant(
	key: string,
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string
): boolean {
	const [kdb, kschema, ktable, kcol, kaction] = key.split('|');
	if (kdb !== db || kaction !== action) return false;
	if (schema === '*') return kschema !== '*';
	if (kschema !== schema) return false;
	if (table === '*') return ktable !== '*';
	if (ktable !== table) return false;
	if (col === '*') return kcol !== '*';
	return false; // column is leaf
}

export function computePermissionChange(
	permissionMap: PermissionMap,
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string
): PermissionChange[] {
	const primary = computePrimaryChange(permissionMap, db, schema, table, col, action);
	const bound = computeBindingChanges(permissionMap, db, schema, table, col, action, primary);
	return [...primary, ...bound];
}

function computePrimaryChange(
	permissionMap: PermissionMap,
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string
): PermissionChange[] {
	const k = permissionKey(db, schema, table, col, action);
	const explicit = permissionMap.get(k);

	// none → allow → deny → none
	if (!explicit) {
		return [{ type: 'add', db, schema, table, col, action, effect: 'allow' }];
	}
	if (explicit.effect === 'allow') {
		const ops: PermissionChange[] = [{ type: 'remove', id: explicit.id }];
		for (const [key, entry] of permissionMap) {
			if (!isDescendant(key, db, schema, table, col, action)) continue;
			ops.push({ type: 'remove', id: entry.id });
		}
		ops.push({ type: 'add', db, schema, table, col, action, effect: 'deny' });
		return ops;
	}
	return [{ type: 'remove', id: explicit.id }];
}

// see implies select: granting see also grants select, denying select revokes see
function computeBindingChanges(
	permissionMap: PermissionMap,
	db: string,
	schema: string,
	table: string,
	col: string,
	action: string,
	primary: PermissionChange[]
): PermissionChange[] {
	const addedAllow = primary.some((c) => c.type === 'add' && c.effect === 'allow');
	const addedDeny = primary.some((c) => c.type === 'add' && c.effect === 'deny');

	if (action === 'see' && addedAllow) {
		const selectKey = permissionKey(db, schema, table, col, 'select');
		const existing = permissionMap.get(selectKey);
		if (existing?.effect === 'allow') return [];
		const ops: PermissionChange[] = [];
		if (existing) ops.push({ type: 'remove', id: existing.id });
		ops.push({ type: 'add', db, schema, table, col, action: 'select', effect: 'allow' });
		return ops;
	}
	if (action === 'select' && addedDeny) {
		const seeKey = permissionKey(db, schema, table, col, 'see');
		const existing = permissionMap.get(seeKey);
		if (existing?.effect === 'allow') {
			return [{ type: 'remove', id: existing.id }];
		}
	}
	return [];
}

// True when node itself not fully allowed but at least one child is
export function isIndeterminate(
	map: PermissionMap,
	db: string,
	schema: string,
	table: string,
	childKeys: string[], // resolved keys for children at the next level
	action: string
): boolean {
	const selfResolved = resolve(map, db, schema, table, '*', action);
	if (selfResolved === 'allow') return false; // fully covered
	return childKeys.some((k) => resolve(map, db, schema, table, k, action) === 'allow');
}
