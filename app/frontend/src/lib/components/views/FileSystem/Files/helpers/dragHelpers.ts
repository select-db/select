import type * as graph from '$lib/wails/graph';

/**
 * Check if a target is a valid drop target
 * A target is valid if it's NOT in the dragged selection
 * (The backend will handle any other filesystem-level errors)
 */
export const isValidDropTarget = (targetId: string, draggedIds: Set<string>): boolean => {
	return !draggedIds.has(targetId);
};

/**
 * Find an item by ID in the tree
 */
export const findItemById = (
	id: string,
	files: graph.FileNode[],
	folders: graph.FolderNode[],
	databases: graph.DBInstanceNode[]
): graph.FileNode | graph.FolderNode | graph.DBInstanceNode | null => {
	// Check files
	for (const file of files) {
		if (file.id === id) return file;
	}

	// Check folders
	for (const folder of folders) {
		if (folder.id === id) return folder;
		const found = findItemById(
			id,
			folder.files,
			folder.folders,
			folder.db_instances
		);
		if (found) return found;
	}

	// Check databases and their contents
	for (const db of databases) {
		if (db.id === id) return db;
		// Search inside database files and folders
		const found = findItemById(id, db.files, db.folders, []);
		if (found) return found;
	}

	return null;
};
