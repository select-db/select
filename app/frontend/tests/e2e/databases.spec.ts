import { call, expect, holdSession, test, type Page } from './wails';
import type { APIRequestContext } from '@playwright/test';
import { labelledInput, tab, testId, treeNode } from './selectors';

/**
 * A database is a directory named after itself. There is nowhere else its name
 * is written down, so everything that changes the name changes the directory,
 * and everything that changes the directory changes the name.
 *
 * This is about that one fact: what a database is called when it is made, what
 * renaming it from the tree and from its own form does to the directory, what
 * moving the directory does to the database, and what the name is allowed to
 * be. The rest of what a database does -- connecting, querying -- is elsewhere.
 *
 * One scenario rather than a test per gesture, for the same reason as
 * filesystem.spec.ts: each step is only meaningful on the state the last left.
 * What it makes it removes, so the seeded workspace ends as it started.
 */

test.setTimeout(180_000);

const FS = 'selectDb/internal/fs_provider.FSProvider';
const GRAPH = 'selectDb/internal/graph.Graph';

/** The tree's own context menu, from the empty space below the last row. */
async function openTreeMenu(page: Page) {
	const panel = testId(page, 'tree.panel');
	const box = await panel.boundingBox();
	if (!box) throw new Error('file tree is not on screen');

	await panel.click({ button: 'right', position: { x: 20, y: box.height - 20 } });
}

async function openMenuOn(page: Page, name: string) {
	await treeNode(page, name).click({ button: 'right' });
}

async function chooseMenuItem(page: Page, name: string) {
	await page.getByRole('menuitem', { name, exact: true }).click();
}

/**
 * The rename box, wherever in the tree it is open.
 *
 * Scoped to the tree: a database's form is open on screen through most of this
 * scenario, and its own Name field is a textbox called Name too.
 */
const renameBox = (page: Page) =>
	testId(page, 'tree.panel').getByRole('textbox', { name: 'Name' });

/**
 * Types a whole new name into the rename box and commits it.
 *
 * The box opens with part of the name selected, which is right for a person and
 * ambiguous for a test -- hence the select-all.
 */
async function renameTo(page: Page, name: string) {
	const box = renameBox(page);
	await expect(box).toBeFocused();

	await box.press('ControlOrMeta+a');
	await box.fill(name);
	await box.press('Enter');
}

/** The names of the databases the graph is holding, in its own order. */
async function databasesInGraph(request: APIRequestContext): Promise<string[]> {
	const workspace = await call<{ db_instances: { name: string }[] }>(
		request,
		`${GRAPH}.GetWorkspaceGraph`
	);

	return (workspace.db_instances ?? []).map((db) => db.name);
}

/** Whether the workspace holds an entry at that path, root-relative. */
async function existsInWorkspace(
	request: APIRequestContext,
	workspaceId: string,
	path: string
): Promise<boolean> {
	const prefix = await call<string>(request, `${FS}.WorkspaceURIPrefix`);

	try {
		await call(request, `${FS}.Stat`, `${prefix}${workspaceId}/${path}`);
		return true;
	} catch {
		return false;
	}
}

/**
 * Runs a command in the workspace root, standing in for everything that changes
 * a workspace without going through the app: a terminal, a git checkout, an
 * editor somebody else has open. The app only finds out by watching.
 */
async function inWorkspace(
	request: APIRequestContext,
	workspaceId: string,
	command: string,
	...args: string[]
) {
	const result = await call<{ exitCode: number; stderr: string }>(request, `${FS}.ExecuteCommand`, {
		workspaceId,
		command,
		args
	});
	if (result.exitCode !== 0) {
		throw new Error(`${command} ${args.join(' ')}: ${result.stderr}`);
	}
}

test('names a database by its directory, and renames the directory with it', async ({
	page,
	request,
	signIn
}) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	const workspace = await call<{ id: string }>(request, `${GRAPH}.GetWorkspaceGraph`);
	await expect(treeNode(page, 'warehouse')).toBeVisible();

	// --- Made ---------------------------------------------------------------

	// A new database is a directory in the workspace root, named for the
	// database rather than for its id.
	await openTreeMenu(page);
	await chooseMenuItem(page, 'New Database...');
	await expect(treeNode(page, 'db #1')).toBeVisible();
	expect(await existsInWorkspace(request, workspace.id, 'db #1')).toBe(true);

	// --- Renamed from the tree ----------------------------------------------

	await openMenuOn(page, 'db #1');
	await chooseMenuItem(page, 'Rename...');
	await expect(renameBox(page)).toBeFocused();

	// A database's config is written while its form is open -- the form saves on
	// a debounce -- and that arrives as a db_instance update like any other. It
	// used to close the rename box, so whatever was being typed went nowhere.
	// Touching the config raises the same update without the wait.
	await inWorkspace(request, workspace.id, 'touch', 'db #1/db.config.json');
	await expect(renameBox(page)).toBeFocused();

	await renameTo(page, 'analytics');

	await expect(treeNode(page, 'analytics')).toBeVisible();
	await expect(treeNode(page, 'db #1')).toHaveCount(0);

	// The directory went with it, both ways: the new name is there and the old
	// one is not left behind.
	expect(await existsInWorkspace(request, workspace.id, 'analytics')).toBe(true);
	expect(await existsInWorkspace(request, workspace.id, 'db #1')).toBe(false);
	expect(await databasesInGraph(request)).toContain('analytics');

	// --- What a name is allowed to be ---------------------------------------

	// A database renames through the same call a folder does, so a name already
	// in the folder is refused on the same terms. The row goes back to saying
	// what it is still called.
	await openMenuOn(page, 'analytics');
	await chooseMenuItem(page, 'Rename...');
	await renameTo(page, 'warehouse');
	await expect(treeNode(page, 'analytics')).toBeVisible();
	expect(await databasesInGraph(request)).toEqual(['analytics', 'warehouse']);

	// --- Renamed from its own form ------------------------------------------

	// The form's Name field is the same rename, so the tree follows it, and so
	// does the tab that is open on the form.
	await openMenuOn(page, 'analytics');
	await chooseMenuItem(page, 'Edit...');
	await expect(tab(page, 'analytics')).toBeVisible();

	const nameField = labelledInput(page, 'Name');
	await nameField.fill('reporting');
	await nameField.blur();

	await expect(treeNode(page, 'reporting')).toBeVisible();
	await expect(treeNode(page, 'analytics')).toHaveCount(0);
	await expect(tab(page, 'reporting')).toBeVisible();
	expect(await existsInWorkspace(request, workspace.id, 'reporting')).toBe(true);

	// A refusal leaves the field saying what the directory is still called,
	// rather than the name that was turned down.
	await nameField.fill('warehouse');
	await nameField.blur();
	await expect(nameField).toHaveValue('reporting');
	await expect(treeNode(page, 'reporting')).toBeVisible();

	// --- Moved --------------------------------------------------------------

	// The directory is a directory: moving it into a folder moves the database,
	// which keeps its name because the move did not change it.
	await openTreeMenu(page);
	await chooseMenuItem(page, 'New folder...');
	await renameTo(page, 'europe');
	await expect(treeNode(page, 'europe')).toBeVisible();

	await inWorkspace(request, workspace.id, 'mv', 'reporting', 'europe/reporting');

	await expect(treeNode(page, 'europe')).toBeVisible();
	await treeNode(page, 'europe').click();
	await expect(treeNode(page, 'reporting')).toBeVisible();
	expect(await existsInWorkspace(request, workspace.id, 'reporting')).toBe(false);
	expect(await databasesInGraph(request)).toContain('reporting');

	// --- Renamed from outside the app ---------------------------------------

	// Nothing tells the app; it finds out by watching, and the name it shows is
	// the directory's whoever changed it.
	await inWorkspace(request, workspace.id, 'mv', 'europe/reporting', 'europe/eu-metrics');
	await expect(treeNode(page, 'eu-metrics')).toBeVisible();
	await expect(treeNode(page, 'reporting')).toHaveCount(0);

	// And the watch went with it: a file made inside the renamed directory
	// still reaches the tree, which it does not if the watch is left pointing
	// at the name the directory had.
	await inWorkspace(request, workspace.id, 'touch', 'europe/eu-metrics/inside.sql');
	await treeNode(page, 'eu-metrics').click();
	await expect(treeNode(page, 'inside.sql')).toBeVisible();

	// --- Deleted ------------------------------------------------------------

	await openMenuOn(page, 'eu-metrics');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'eu-metrics')).toHaveCount(0);
	await expect(tab(page, 'eu-metrics')).toHaveCount(0);

	// --- Leaving it as it was found -----------------------------------------

	await openMenuOn(page, 'europe');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'europe')).toHaveCount(0);

	expect(await databasesInGraph(request)).toEqual(['warehouse']);
	await expect(treeNode(page, 'warehouse')).toBeVisible();
});
