import { ResolveFolder } from '$lib/wails/graph';

// The graph carries every folder but only the files of the ones that have been
// opened, so opening one asks the backend to read it. Fire-and-forget: the files
// arrive with the next graph update.

/** Only these ids name a folder on disk; "search::…" and "git::…" ids do not. */
const WORKSPACE_URI_PREFIX = 'selectdb://workspaces/';

/** Folders already asked for. Expanding is frequent, the answer is not. */
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
