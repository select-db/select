import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
import * as graph from '$lib/wails/graph';
import { revokeConnections } from '$lib/components/views/shared/revokeConnections';
import { must, tryCatch } from '$lib/utils/tryCatch';

export type Entry = graph.FileNode | graph.FolderNode | graph.DBInstanceNode;

/** Removes one entry from the workspace, and nothing else. */
export const removeEntry = async (entry: Entry) =>
	must(tryCatch(fs.Delete, { uri: entry.uri, recursive: entry.type !== 'file' }));

/**
 * Deletes entries from the workspace, revoking any shared connection that goes
 * with them.
 *
 * A local database is a directory and nothing else, so deleting it is deleting
 * the directory. A proxified one also has a credential on the server, and that
 * credential is what lets anyone in the workspace query it. Removing the
 * directory does not touch it: the folder is replicated through git, so its
 * absence is local, revertible, and reaches other people only when they pull,
 * while the credential is one row that either exists or does not.
 *
 * Every delete route comes through here on purpose -- a database deleted
 * directly, a folder deleted with one inside it, a multi-selection with one
 * among it -- so the rule cannot be walked around by choosing a different
 * gesture.
 *
 * onDeleted runs once the entries are actually gone. A shared connection puts a
 * confirmation in the way, so a caller that announced the deletion on its own
 * would be announcing something the person can still cancel.
 *
 * The revokes go first, and any refusal abandons the whole delete. A revoke
 * that failed after the directory was gone would strand the credential: see
 * datasource.ListHandler for why the file is the only thing still naming it.
 */
export const deleteEntries = async (entries: Entry[], onDeleted?: () => void): Promise<void> => {
	const shared = await must(
		tryCatch(
			graph.SharedDatabasesUnder,
			entries.map((entry) => entry.id)
		)
	);

	if (!(await revokeConnections(shared, 'Delete'))) return;

	for (const entry of entries) await removeEntry(entry);
	onDeleted?.();
};
