import { WorkspaceURIPrefix } from '$lib/bindings/selectDb/internal/fs_provider/fsprovider';

/**
 * The prefix every workspace URI starts with, from the Go side that defines it.
 *
 * Asked once and remembered: it cannot change while the app is running, and the
 * request is started as soon as this module is imported so the first caller
 * rarely waits for it.
 */
let prefix: Promise<string> | undefined;

export function workspaceUriPrefix(): Promise<string> {
	prefix ??= WorkspaceURIPrefix().then(
		(value) => value,
		// A failed call must not be remembered as the answer; the next caller
		// asks again.
		(error) => {
			prefix = undefined;
			throw error;
		}
	);
	return prefix;
}

/**
 * Whether an id names a file or folder on disk.
 *
 * The trees render more than the workspace: a search result ("search::file::…")
 * and a git status row ("git::staged") carry ids that name neither.
 */
export async function isWorkspaceUri(id: string): Promise<boolean> {
	return id.startsWith(await workspaceUriPrefix());
}

void workspaceUriPrefix().catch(() => {});
