import type { Component } from 'svelte';
import { get } from 'svelte/store';

import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
import { DeleteDatasource } from '$lib/bindings/selectDb/internal/datasource/datasource';
import type * as graph from '$lib/wails/graph';
import RevokeConnectionModal from '$lib/components/views/Database/RevokeConnectionModal.svelte';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { myPermissions } from '$lib/stores/myPermissionsStore';
import { must, tryCatch } from '$lib/utils/tryCatch';

type Entry = graph.FileNode | graph.FolderNode | graph.DBInstanceNode;

/**
 * Removes one entry from the workspace, and nothing else.
 *
 * A query file carries a sibling holding its result metadata, which has no row
 * of its own in the tree and would otherwise be left behind.
 */
export const removeEntry = async (entry: Entry) => {
	if (entry.type === 'file') {
		await must(tryCatch(fs.Delete, { uri: entry.uri, recursive: false }));
		await tryCatch(fs.Delete, { uri: entry.uri + '.metadata.json', recursive: false });
		return;
	}
	await must(tryCatch(fs.Delete, { uri: entry.uri, recursive: true }));
};

/** Every proxified database at or under an entry, however deep. */
const sharedDatabasesIn = (entry: Entry): graph.DBInstanceNode[] => {
	if (entry.type === 'db_instance') {
		const db = entry as graph.DBInstanceNode;
		return db.proxified ? [db] : [];
	}
	if (!('db_instances' in entry)) return [];

	const folder = entry as graph.FolderNode;
	return [
		...(folder.db_instances ?? []).filter((db) => db?.proxified),
		...(folder.folders ?? []).flatMap((child) => (child ? sharedDatabasesIn(child) : []))
	] as graph.DBInstanceNode[];
};

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
 * Every route into here is covered on purpose -- a database deleted directly, a
 * folder deleted with one inside it, a multi-selection with one among it -- so
 * the rule cannot be walked around by choosing a different gesture.
 *
 * onDeleted runs once the entries are actually gone. A shared connection puts a
 * confirmation in the way, so a caller that announced the deletion on its own
 * would be announcing something the person can still cancel.
 *
 * The revokes go first, and any refusal abandons the whole delete. A revoke
 * that failed after the directory was gone would leave a credential nobody can
 * see any more, because the id it is stored under lived in the file that was
 * just deleted.
 */
export const deleteEntries = async (entries: Entry[], onDeleted?: () => void): Promise<void> => {
	const shared = entries.flatMap(sharedDatabasesIn);

	if (shared.length === 0) {
		for (const entry of entries) await removeEntry(entry);
		onDeleted?.();
		return;
	}

	// Credentials are administrated, not owned by whoever has the workspace open.
	// The server refuses this too; refusing here is so the answer arrives before
	// anything has been deleted.
	const { canManageDb } = get(myPermissions);
	const refused = shared.filter((db) => !canManageDb(db.id));
	if (refused.length > 0) {
		notifyError(
			`${listNames(refused)} ${refused.length === 1 ? 'is a shared connection' : 'are shared connections'}. Ask an admin to delete ${refused.length === 1 ? 'it' : 'them'}, so the stored credentials go too.`
		);
		return;
	}

	modalStore.set({
		content: (() => RevokeConnectionModal) as () => Component,
		width: 520,
		props: {
			names: shared.map((db) => db.name),
			onCancel: () => modalStore.set(null),
			onConfirm: async () => {
				modalStore.set(null);
				await revokeThenDelete(entries, shared, onDeleted);
			}
		}
	});
};

const revokeThenDelete = async (
	entries: Entry[],
	shared: graph.DBInstanceNode[],
	onDeleted?: () => void
) => {
	for (const db of shared) {
		const [, err] = await tryCatch(DeleteDatasource, db.id);
		if (err) {
			// Deliberately leaves everything on disk. The workspace files are the
			// only thing still naming these connections, and losing them here would
			// hide what failed.
			notifyError(
				`Nothing was deleted: the stored credentials for ${db.name} could not be revoked.`
			);
			return;
		}
	}

	for (const entry of entries) await removeEntry(entry);
	onDeleted?.();
};

const listNames = (dbs: graph.DBInstanceNode[]) => dbs.map((db) => db.name).join(', ');
