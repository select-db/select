import type { graph } from '$lib/wailsjs/go/models';

export interface ResolvedDbInstance {
	name: string;
	dbType: string;
}

function collect(
	folders: graph.FolderNode[] | undefined,
	databases: graph.DBInstanceNode[] | undefined,
	out: Map<string, ResolvedDbInstance>
): void {
	for (const db of databases ?? []) {
		out.set(db.id, { name: db.name, dbType: db.db_type });
	}
	for (const folder of folders ?? []) {
		collect(folder.folders, folder.db_instances, out);
	}
}

/**
 * Builds an id -> {name, dbType} lookup by walking the workspace graph tree
 * (root databases plus databases nested in folders). Used to label history
 * rows; ids missing from the map belong to removed instances.
 */
export function buildDbInstanceLookup(
	workspace: graph.WorkspaceNode | undefined
): Map<string, ResolvedDbInstance> {
	const lookup = new Map<string, ResolvedDbInstance>();
	if (!workspace) return lookup;
	collect(workspace.folders, workspace.db_instances, lookup);
	return lookup;
}
