import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
import { updateFileTabsAfterRename } from '$lib/components/Layout/layoutStore';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { tryCatch } from '$lib/utils/tryCatch';

/**
 * Renames a workspace entry to another name in the same folder, and repoints
 * whatever tabs were showing it.
 *
 * Returns the URI it ended up at, or null if the rename was refused -- a name
 * the folder already holds, or one the filesystem will not take. The caller
 * puts its field back to what the entry is still called.
 *
 * Built from the URI rather than an id: a file or folder answers to its own
 * path, but a database answers to the id in its config, which is not one.
 */
export const renameEntry = async (uri: string, name: string): Promise<string | null> => {
	const newUri = `${uri.slice(0, uri.lastIndexOf('/') + 1)}${name}`;
	if (newUri === uri) return uri;

	const [, err] = await tryCatch(fs.Rename, { old_uri: uri, new_uri: newUri });
	if (err) {
		notifyError(err.message);
		return null;
	}

	updateFileTabsAfterRename(uri, newUri, name);
	return newUri;
};
