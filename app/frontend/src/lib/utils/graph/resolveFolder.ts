import { ResolveFolder } from '$lib/wails/graph';

/**
 * Opening a folder in the tree asks the backend to read its files: the graph
 * carries every folder but only the files of the ones that have been opened.
 *
 * Fire-and-forget — the files arrive with the next graph update, like any other
 * change to the workspace — and asked once per folder, since expanding is a
 * frequent gesture and the answer does not change.
 *
 * Only workspace URIs name folders on disk. The search and git panels render
 * trees whose ids ("search::file::…", "git::staged") name results, not
 * directories, and are left alone.
 */
const WORKSPACE_URI_PREFIX = 'selectdb://workspaces/';

const asked = new Set<string>();

export function resolveFolderContents(id: string) {
	if (!id.startsWith(WORKSPACE_URI_PREFIX) || asked.has(id)) return;

	asked.add(id);
	void ResolveFolder(id).catch(() => {
		// Let the next open try again rather than leaving the folder stuck empty
		// on a transient failure.
		asked.delete(id);
	});
}

/** Drops what has been asked for, so a reloaded workspace asks again. */
export function clearResolvedFolders() {
	asked.clear();
}
