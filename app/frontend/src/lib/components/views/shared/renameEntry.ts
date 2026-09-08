import * as fs from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';
import { updateFileTabsAfterRename } from '$lib/components/Layout/layoutStore';
import { notifyError } from '$lib/system/Notifications/notificationsStore';
import { tryCatch } from '$lib/utils/tryCatch';

/**
 * Renames a workspace entry to another name in the same folder, repoints
 * whatever tabs were showing it, and says whether it was allowed. A refusal --
 * a name the folder already holds, or one the filesystem will not take -- is
 * the caller's cue to put its field back.
 *
 * Keyed on the URI rather than an id: a file or folder answers to its own path,
 * but a database answers to the id in its config, which is not one.
 */
export const renameEntry = async (uri: string, name: string): Promise<boolean> => {
	const newUri = `${uri.slice(0, uri.lastIndexOf('/') + 1)}${name}`;
	if (newUri === uri) return true;

	const [, err] = await tryCatch(fs.Rename, { old_uri: uri, new_uri: newUri });
	if (err) {
		notifyError(err.message);
		return false;
	}

	updateFileTabsAfterRename(uri, newUri, name);
	return true;
};
