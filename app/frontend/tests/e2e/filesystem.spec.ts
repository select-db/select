import { call, expect, holdSession, test, type Page } from './wails';
import type { APIRequestContext } from '@playwright/test';
import { editor, tab, testId, treeNode } from './selectors';

/**
 * File management: what a person does to the workspace tree in a session —
 * make folders, put files in them, name them, bind one to a database, move
 * them, delete them — plus the things that happen to a workspace while the app
 * is only watching: a file removed in a terminal, one restored by git, one
 * appearing in a folder nobody has opened.
 *
 * One scenario rather than a test per gesture. These steps are what the app
 * does in sequence, and each is only meaningful on the state the last one left:
 * a rename is interesting because there is something beside it to disturb, a
 * delete because a tab is open on what is being deleted, a move because the
 * folder it lands in can be collapsed again to prove it went there.
 *
 * Everything it makes, it removes, so the seeded workspace is unchanged at the
 * end — the same workspace the screenshot suite photographs and the other specs
 * read.
 */

/** Long by design: one session's worth of gestures, each waiting on the app. */
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

/** The context menu of one row. */
async function openMenuOn(page: Page, name: string) {
	await treeNode(page, name).click({ button: 'right' });
}

/**
 * Picks an entry from whichever menu is open.
 *
 * Exactly, because the tree's menus carry both "Delete" and "Delete selected":
 * a loose match on the first would sometimes hit the second and take rows with
 * it that nobody named.
 */
async function chooseMenuItem(page: Page, name: string) {
	await page.getByRole('menuitem', { name, exact: true }).click();
}

/**
 * Leaves exactly the named rows selected.
 *
 * Ctrl-click toggles, so a sequence of them only means what it says from a
 * known starting point. Clicking a folder sets the selection to that folder --
 * and toggles it open, hence the second click putting it back -- and
 * ctrl-clicking it off then leaves nothing selected.
 */
async function selectOnly(page: Page, folder: string, ...rows: string[]) {
	await treeNode(page, folder).click();
	await treeNode(page, folder).click();
	await treeNode(page, folder).click({ modifiers: ['ControlOrMeta'] });
	for (const row of rows) await treeNode(page, row).click({ modifiers: ['ControlOrMeta'] });
}

const renameBox = (page: Page) => page.getByRole('textbox', { name: 'Name' });

/**
 * Types into the rename box a new file, a new folder and "Rename..." all open,
 * and commits it.
 *
 * The box opens with part of the name selected — up to the extension, so typing
 * keeps it — which is right for a person and ambiguous for a test, hence the
 * select-all first: what is typed here is the whole new name.
 */
async function renameTo(page: Page, name: string) {
	const box = renameBox(page);
	await expect(box).toBeFocused();

	await box.press('ControlOrMeta+a');
	await box.fill(name);
	await box.press('Enter');

	await expect(box).toBeHidden();
}

/** Leaves the rename box without renaming, which is how a default name is kept. */
async function keepName(page: Page) {
	const box = renameBox(page);
	await expect(box).toBeFocused();
	await box.press('Escape');
	await expect(box).toBeHidden();
}

/** Finds a file by name in the workspace picker and opens it. */
async function findInPicker(page: Page, name: string) {
	await page.keyboard.press('ControlOrMeta+p');
	await page.getByPlaceholder('Search workspace...').fill(name);

	const hit = page.getByRole('menuitem').filter({ hasText: name });
	await expect(hit).toBeVisible();
	await hit.click();
}

type GraphFolder = { name: string; folders: GraphFolder[] };

/** Whether the graph is holding a folder of that name, at any depth. */
async function folderInGraph(request: APIRequestContext, name: string): Promise<boolean> {
	const workspace = await call<{ folders: GraphFolder[] }>(request, `${GRAPH}.GetWorkspaceGraph`);

	const holds = (folders: GraphFolder[]): boolean =>
		folders.some((folder) => folder.name === name || holds(folder.folders ?? []));

	return holds(workspace.folders ?? []);
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

test('creates, renames, moves and deletes files and folders', async ({ page, request, signIn }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	// What the workspace starts as.
	await expect(treeNode(page, 'weekly_revenue.sql')).toBeVisible();

	const workspace = await call<{ id: string }>(request, `${GRAPH}.GetWorkspaceGraph`);
	const run = (command: string, ...args: string[]) =>
		inWorkspace(request, workspace.id, command, ...args);

	// --- Making things -------------------------------------------------------

	// A new folder is created named, and named again by the person making it.
	await openTreeMenu(page);
	await chooseMenuItem(page, 'New folder...');
	await renameTo(page, 'reports');
	await expect(treeNode(page, 'reports')).toBeVisible();

	// And folders nest.
	await openMenuOn(page, 'reports');
	await chooseMenuItem(page, 'New folder...');
	await renameTo(page, '2026');
	await expect(treeNode(page, '2026')).toBeVisible();

	// A file made two folders deep opens as a tab, and what is typed into it is
	// written to disk without a save.
	await openMenuOn(page, '2026');
	await chooseMenuItem(page, 'New file...');
	await renameTo(page, 'daily.sql');
	await expect(treeNode(page, 'daily.sql')).toBeVisible();
	await expect(tab(page, 'daily.sql')).toBeVisible();

	await editor.surface(page).click();
	await page.keyboard.type('SELECT 1;');
	await expect(editor.line(page, 'SELECT 1;')).toBeVisible();

	// A file is bound to a database from the picker; the binding is a sidecar
	// beside the file, and the header names it.
	await page.keyboard.press('ControlOrMeta+Shift+d');
	const dbPicker = page.getByPlaceholder('Search db...');
	await expect(dbPicker).toBeVisible();
	await page.getByText('warehouse', { exact: true }).last().click();
	await page.keyboard.press('Escape');
	await expect(dbPicker).toBeHidden();
	await expect(page.getByRole('button', { name: 'warehouse' })).toBeVisible();

	// Two more files, keeping the names the app proposes. Each name is unique or
	// the second would land on the first — FSProvider.Write truncates — and each
	// gets its own content, so the rename below can be shown not to move it.
	const proposed = [
		{ name: '#1.sql', content: 'SELECT 1 AS one;' },
		{ name: '#2.sql', content: 'SELECT 2 AS two;' }
	];
	for (const file of proposed) {
		await openMenuOn(page, 'reports');
		await chooseMenuItem(page, 'New file...');
		await keepName(page);
		await expect(treeNode(page, file.name)).toBeVisible();

		await editor.surface(page).click();
		await page.keyboard.type(file.content);
		await expect(editor.line(page, file.content)).toBeVisible();
	}

	// --- Renaming ------------------------------------------------------------

	// Renaming moves the file on disk. The row takes the new name, the tab
	// follows the file rather than closing on a path that no longer exists, and
	// the rest of the workspace stays where it was — a rename rebuilds the whole
	// graph, and the folders that had been read have to come back read.
	await openMenuOn(page, 'daily.sql');
	await chooseMenuItem(page, 'Rename...');
	await renameTo(page, 'weekly.sql');

	await expect(treeNode(page, 'weekly.sql')).toBeVisible();
	await expect(treeNode(page, 'daily.sql')).toHaveCount(0);
	await expect(treeNode(page, '#1.sql')).toBeVisible();
	await expect(treeNode(page, '#2.sql')).toBeVisible();

	// It still holds what was typed into it under its old name, and the database
	// bound to it: the sidecar moved with the file.
	await tab(page, 'weekly.sql').click();
	await expect(editor.line(page, 'SELECT 1;')).toBeVisible();
	await expect(page.getByRole('button', { name: 'warehouse' })).toBeVisible();

	// Escape leaves a name alone.
	await openMenuOn(page, '#2.sql');
	await chooseMenuItem(page, 'Rename...');
	await renameBox(page).fill('escaped.sql');
	await keepName(page);
	await expect(treeNode(page, '#2.sql')).toBeVisible();
	await expect(treeNode(page, 'escaped.sql')).toHaveCount(0);

	// A rename onto a name already in the folder is refused rather than
	// overwriting it: os.Rename replaces its target without a word.
	await openMenuOn(page, '#2.sql');
	await chooseMenuItem(page, 'Rename...');
	await renameTo(page, '#1.sql');

	await expect(treeNode(page, '#1.sql')).toBeVisible();
	await expect(treeNode(page, '#2.sql')).toBeVisible();

	// Neither file moved: the one that would have been overwritten still holds
	// its own content, not the other's.
	await tab(page, '#1.sql').click();
	await expect(editor.line(page, 'SELECT 1 AS one;')).toBeVisible();
	await tab(page, '#2.sql').click();
	await expect(editor.line(page, 'SELECT 2 AS two;')).toBeVisible();

	// --- Moving --------------------------------------------------------------

	// Dropping a file on a folder moves it there. Collapsing the folder takes
	// the file with it, which is what proves where it landed.
	await treeNode(page, '#2.sql').dragTo(treeNode(page, '2026'));
	await expect(treeNode(page, '#2.sql')).toBeVisible();

	await treeNode(page, '2026').click();
	await expect(treeNode(page, '#2.sql')).toHaveCount(0);
	await treeNode(page, '2026').click();
	await expect(treeNode(page, '#2.sql')).toBeVisible();

	// Dropping it back where it already is asks to rename it to its own name.
	// Nothing moves, nothing is lost, and nothing is said about it.
	await treeNode(page, '#2.sql').dragTo(treeNode(page, '2026'));
	await expect(treeNode(page, '#2.sql')).toBeVisible();
	await tab(page, '#2.sql').click();
	await expect(editor.line(page, 'SELECT 2 AS two;')).toBeVisible();

	// A drop onto a folder that already has a file of that name is refused, the
	// same as typing the name would be. Two files called the same thing, one in
	// each folder:
	for (const [folder, content] of [
		['reports', 'SELECT 4 AS four;'],
		['2026', 'SELECT 5 AS five;']
	]) {
		await openMenuOn(page, folder);
		await chooseMenuItem(page, 'New file...');
		await renameTo(page, 'twin.sql');
		await editor.surface(page).click();
		await page.keyboard.type(content);
		await expect(editor.line(page, content)).toBeVisible();
	}
	await expect(treeNode(page, 'twin.sql')).toHaveCount(2);

	// Closing 2026 leaves one of them on screen, which is the one to drag — and
	// after the refusal there are still two.
	await treeNode(page, '2026').click();
	await expect(treeNode(page, 'twin.sql')).toHaveCount(1);
	await treeNode(page, 'twin.sql').dragTo(treeNode(page, '2026'));
	await expect(treeNode(page, 'twin.sql')).toBeVisible();

	// Deleting that one while 2026 is still closed leaves the tree with none of
	// them on screen, so the one that comes back when 2026 opens is the other.
	await openMenuOn(page, 'twin.sql');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'twin.sql')).toHaveCount(0);

	await treeNode(page, '2026').click();
	await expect(treeNode(page, 'twin.sql')).toBeVisible();
	await tab(page, 'twin.sql').click();
	await expect(editor.line(page, 'SELECT 5 AS five;')).toBeVisible();

	// A database directory is a folder on disk, and takes a drop like one.
	await treeNode(page, '#1.sql').dragTo(treeNode(page, 'warehouse'));
	await treeNode(page, 'warehouse').click();
	await expect(treeNode(page, '#1.sql')).toBeVisible();
	await treeNode(page, 'warehouse').click();
	await expect(treeNode(page, '#1.sql')).toHaveCount(0);

	// The empty space below the tree is the workspace root, and takes it back
	// out again: it stays on screen when the database is closed.
	await treeNode(page, 'warehouse').click();
	await treeNode(page, '#1.sql').dragTo(
		page.getByRole('region', { name: 'File system root drop zone' })
	);
	await treeNode(page, 'warehouse').click();
	await expect(treeNode(page, '#1.sql')).toBeVisible();

	// Several rows selected move together: dragging one of them takes the rest.
	await selectOnly(page, 'reports', '#1.sql', 'twin.sql');
	await treeNode(page, '#1.sql').dragTo(treeNode(page, 'reports'));

	await treeNode(page, 'reports').click();
	await expect(treeNode(page, '#1.sql')).toHaveCount(0);
	await expect(treeNode(page, 'twin.sql')).toHaveCount(0);
	await treeNode(page, 'reports').click();
	await expect(treeNode(page, '#1.sql')).toBeVisible();
	await expect(treeNode(page, 'twin.sql')).toBeVisible();

	// twin.sql has served its purpose; #1.sql is still needed below. A drag
	// leaves what it moved selected, so the selection is named again first.
	await selectOnly(page, '2026', 'twin.sql');
	await openMenuOn(page, 'twin.sql');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'twin.sql')).toHaveCount(0);

	// --- Deleting ------------------------------------------------------------

	// More than one row selected turns the menu into a batch delete, across
	// folders.
	await selectOnly(page, '2026', '#1.sql', '#2.sql');
	await openMenuOn(page, '#2.sql');
	await chooseMenuItem(page, 'Delete selected');

	await expect(treeNode(page, '#1.sql')).toHaveCount(0);
	await expect(treeNode(page, '#2.sql')).toHaveCount(0);
	await expect(tab(page, '#1.sql')).toHaveCount(0);
	await expect(tab(page, '#2.sql')).toHaveCount(0);

	// --- Names that have to be refused --------------------------------------

	// A name of nothing but spaces would rename the file onto the folder it
	// sits in, and a name climbing out of the workspace would put it on the
	// host filesystem. Both leave the file where it is.
	for (const refused of ['   ', '../../../escaped.sql']) {
		await openMenuOn(page, 'weekly.sql');
		await chooseMenuItem(page, 'Rename...');
		await renameTo(page, refused);
		await expect(treeNode(page, 'weekly.sql')).toBeVisible();
	}
	await expect(treeNode(page, 'escaped.sql')).toHaveCount(0);

	// --- Renaming and moving a folder ---------------------------------------

	// A folder rename moves everything under it. The rows come back under the
	// new name, and a file open in a tab is still readable at its new path
	// rather than pointing at one that no longer exists.
	await openMenuOn(page, 'reports');
	await chooseMenuItem(page, 'New folder...');
	await renameTo(page, 'before');
	await openMenuOn(page, 'before');
	await chooseMenuItem(page, 'New file...');
	await renameTo(page, 'inside.sql');
	await editor.surface(page).click();
	await page.keyboard.type('SELECT 3 AS three;');
	await expect(editor.line(page, 'SELECT 3 AS three;')).toBeVisible();

	await openMenuOn(page, 'before');
	await chooseMenuItem(page, 'Rename...');
	await renameTo(page, 'after');

	await expect(treeNode(page, 'after')).toBeVisible();
	await expect(treeNode(page, 'before')).toHaveCount(0);

	// The file went with the folder: it is still in the workspace, and the tab
	// open on it still reads it at its new path rather than pointing at one that
	// no longer exists.
	//
	// Asked for rather than looked for in the tree: whether a renamed folder
	// comes back open or closed depends on whether its files were announced
	// again on the way, and that is not what this is about.
	await findInPicker(page, 'inside.sql');
	await tab(page, 'inside.sql').click();
	await expect(editor.line(page, 'SELECT 3 AS three;')).toBeVisible();

	// Folders drag like files do.
	await treeNode(page, 'after').dragTo(treeNode(page, '2026'));
	await treeNode(page, '2026').click();
	await expect(treeNode(page, 'after')).toHaveCount(0);
	await treeNode(page, '2026').click();
	await expect(treeNode(page, 'after')).toBeVisible();

	// --- Selecting a range ---------------------------------------------------

	// Shift-click takes everything between the two rows, which the menu reports
	// by offering the batch delete. Nothing is deleted here: these are the
	// workspace's own files.
	await treeNode(page, 'top_customers.sql').click();
	await treeNode(page, 'weekly_revenue.sql').click({ modifiers: ['Shift'] });
	await openMenuOn(page, 'weekly_revenue.sql');
	await expect(page.getByRole('menuitem', { name: 'Delete selected' })).toBeVisible();
	await page.keyboard.press('Escape');

	// And the selection is put back to one row, or every menu after this one is
	// the batch delete.
	await treeNode(page, 'reports').click();
	await treeNode(page, 'reports').click();
	await treeNode(page, 'reports').click({ modifiers: ['ControlOrMeta'] });

	// --- Files inside a database --------------------------------------------

	// A database is a directory too: files can be made in it, and they are the
	// database's own rather than the folder's.
	await openMenuOn(page, 'warehouse');
	await chooseMenuItem(page, 'New file...');
	await renameTo(page, 'in-db.sql');
	await expect(treeNode(page, 'in-db.sql')).toBeVisible();

	await openMenuOn(page, 'in-db.sql');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'in-db.sql')).toHaveCount(0);

	// --- What happens without the app ---------------------------------------

	// A file removed in a terminal leaves the tree and closes its tab. Nothing
	// told the app what it was: the path is gone by the time the event arrives,
	// and a file reported as a folder would leave the tab open on nothing.
	await run('rm', 'reports/2026/weekly.sql', 'reports/2026/weekly.sql.metadata.json');
	await expect(treeNode(page, 'weekly.sql')).toHaveCount(0);
	await expect(tab(page, 'weekly.sql')).toHaveCount(0);

	// A file appearing in a folder nobody has opened is still findable: the
	// picker asks the backend rather than filtering what the tree happens to
	// hold.
	await run('mkdir', '-p', 'archive/quarterly');
	await expect(treeNode(page, 'archive')).toBeVisible();

	// The file is written only once the graph is holding the folder it goes in.
	// A watch is registered per directory as the directory turns up, so a file
	// written into one the watcher has not reached yet is a change nobody sees.
	await expect.poll(() => folderInGraph(request, 'quarterly')).toBe(true);
	await run('cp', 'cohorts.sql', 'archive/quarterly/b.sql');

	await findInPicker(page, 'b.sql');
	await expect(tab(page, 'b.sql')).toBeVisible();

	// A seeded file removed and then restored by git comes back on its own.
	await run('rm', 'cohorts.sql');
	await expect(treeNode(page, 'cohorts.sql')).toHaveCount(0);
	await run('git', 'checkout', '--', 'cohorts.sql');
	await expect(treeNode(page, 'cohorts.sql')).toBeVisible();

	// --- Databases are folders too ------------------------------------------

	// A database is a directory carrying a db.config.json, so it is made and
	// removed like a folder while being a different kind of node.
	await openTreeMenu(page);
	await chooseMenuItem(page, 'New Database...');
	await expect(treeNode(page, 'db #1')).toBeVisible();

	// Databases are listed by name, so the new one lands above the seeded one
	// and pushes it down. Both are waited for before anything is clicked: acting
	// while the rows are still moving hits whichever one arrives under the
	// pointer.
	await expect(treeNode(page, 'warehouse')).toBeVisible();

	await openMenuOn(page, 'db #1');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'db #1')).toHaveCount(0);
	await expect(treeNode(page, 'warehouse')).toBeVisible();

	// --- Leaving it as it was found -----------------------------------------

	// Deleting a folder takes what is still inside it.
	await openMenuOn(page, 'reports');
	await chooseMenuItem(page, 'Delete');
	await expect(treeNode(page, 'reports')).toHaveCount(0);
	await expect(treeNode(page, '2026')).toHaveCount(0);

	await run('rm', '-rf', 'archive');
	await expect(treeNode(page, 'archive')).toHaveCount(0);

	for (const seeded of ['weekly_revenue.sql', 'top_customers.sql', 'cohorts.sql', 'warehouse']) {
		await expect(treeNode(page, seeded)).toBeVisible();
	}
});
