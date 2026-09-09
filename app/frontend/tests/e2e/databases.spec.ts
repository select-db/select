import {
	databasesInGraph,
	existsInWorkspace,
	expect,
	holdSession,
	inWorkspace,
	open,
	test,
	workspaceId,
	type Page
} from './wails';
import { dbStatus, labelledInput, modal, renameBox, tab, testId, treeNode } from './selectors';
import { chooseMenuItem, openMenuOn, openTreeMenu, renameTo } from './tree';

/**
 * A database is a directory named after itself. There is nowhere else its name
 * is written down, so everything that changes the name changes the directory,
 * and everything that changes the directory changes the name.
 *
 * The first scenario is about that one fact: what a database is called when it
 * is made, what renaming it from the tree and from its own form does to the
 * directory, what moving the directory does to the database, and what the name
 * is allowed to be. One scenario rather than a test per gesture, for the same
 * reason as filesystem.spec.ts: each step is only meaningful on the state the
 * last left.
 *
 * The second is about the other thing a row says: whether the database answers.
 *
 * Both leave the seeded workspace as they found it, so they can run in either
 * order and beside every other spec.
 */

test.setTimeout(180_000);

/** The id the sample workspace gives its one database (internal/sample). */
const WAREHOUSE = 'sample-warehouse';

const testConnection = (page: Page) =>
	page.getByRole('button', { name: 'Test connection' }).click();

test('names a database by its directory, and renames the directory with it', async ({
	page,
	request,
	signIn
}) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	const id = await workspaceId(request);
	await expect(treeNode(page, 'warehouse')).toBeVisible();

	// --- Made ---------------------------------------------------------------

	// A new database is a directory in the workspace root, named for the
	// database rather than for its id.
	await openTreeMenu(page);
	await chooseMenuItem(page, 'New Database...');
	await expect(treeNode(page, 'db #1')).toBeVisible();
	expect(await existsInWorkspace(request, id, 'db #1')).toBe(true);

	// --- Renamed from the tree ----------------------------------------------

	await openMenuOn(page, 'db #1');
	await chooseMenuItem(page, 'Rename...');
	await expect(renameBox(page)).toBeFocused();

	// A database's config is written while its form is open -- the form saves on
	// a debounce -- and that arrives as a db_instance update like any other. It
	// used to close the rename box, so whatever was being typed went nowhere.
	// Touching the config raises the same update without the wait.
	await inWorkspace(request, id, 'touch', 'db #1/db.config.json');
	await expect(renameBox(page)).toBeFocused();

	await renameTo(page, 'analytics');

	await expect(treeNode(page, 'analytics')).toBeVisible();
	await expect(treeNode(page, 'db #1')).toHaveCount(0);

	// The directory went with it, both ways: the new name is there and the old
	// one is not left behind.
	expect(await existsInWorkspace(request, id, 'analytics')).toBe(true);
	expect(await existsInWorkspace(request, id, 'db #1')).toBe(false);
	expect(await databasesInGraph(request)).toContain('analytics');

	// --- What a name is allowed to be ---------------------------------------

	// A database renames through the same call a folder does, so a name already
	// in the folder is refused on the same terms. The row goes back to saying
	// what it is still called.
	await openMenuOn(page, 'analytics');
	await chooseMenuItem(page, 'Rename...');

	// Typed here rather than through renameTo, so a config write can land between
	// the typing and the Enter that commits it. The row re-renders, and the guard
	// that swallows the Enter opening the box used to be re-armed by that
	// re-render -- so the Enter below went nowhere and the box could no longer be
	// committed or closed from the keyboard.
	const box = renameBox(page);
	await expect(box).toBeFocused();
	await box.press('ControlOrMeta+a');
	await box.fill('warehouse');
	await inWorkspace(request, id, 'mv', 'warehouse', 'depot');
	await expect(treeNode(page, 'depot')).toBeVisible();
	await inWorkspace(request, id, 'mv', 'depot', 'warehouse');
	await expect(treeNode(page, 'warehouse')).toBeVisible();
	await box.press('Enter');
	await expect(box).toBeHidden();

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
	expect(await existsInWorkspace(request, id, 'reporting')).toBe(true);

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

	await inWorkspace(request, id, 'mv', 'reporting', 'europe/reporting');

	await expect(treeNode(page, 'europe')).toBeVisible();
	await treeNode(page, 'europe').click();
	await expect(treeNode(page, 'reporting')).toBeVisible();
	expect(await existsInWorkspace(request, id, 'reporting')).toBe(false);
	expect(await databasesInGraph(request)).toContain('reporting');

	// --- Renamed from outside the app ---------------------------------------

	// Nothing tells the app; it finds out by watching, and the name it shows is
	// the directory's whoever changed it.
	await inWorkspace(request, id, 'mv', 'europe/reporting', 'europe/eu-metrics');
	await expect(treeNode(page, 'eu-metrics')).toBeVisible();
	await expect(treeNode(page, 'reporting')).toHaveCount(0);

	// And the watch went with it: a file made inside the renamed directory
	// still reaches the tree, which it does not if the watch is left pointing
	// at the name the directory had.
	await inWorkspace(request, id, 'touch', 'europe/eu-metrics/inside.sql');
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

test('shows what the last attempt to reach a database found', async ({ page, signIn }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	await expect(treeNode(page, 'warehouse')).toBeVisible();

	// The availability watcher pings on its own every few seconds, so the dot
	// arrives at "online" without anyone asking. That is the state the rest of
	// this test moves away from and back to.
	// Scoped to the tree: the same indicator appears wherever a database is named
	// -- an open tab, a results badge -- and the tree is where it went stale.
	const dot = testId(page, 'tree.panel').locator(dbStatus(page, WAREHOUSE));
	await expect(dot).toHaveAttribute('data-test-state', 'online', { timeout: 30_000 });

	await openMenuOn(page, 'warehouse');
	await chooseMenuItem(page, 'Edit...');

	const dsn = testId(page, 'database.dsn').locator('input');
	await expect(dsn).toBeVisible();
	const seeded = await dsn.inputValue();

	// --- Refused --------------------------------------------------------------

	// A directory that does not exist, so SQLite can neither open a file there
	// nor create one.
	await dsn.fill('/nonexistent-directory/warehouse.db');
	await dsn.blur();
	await testConnection(page);

	// The row is still collapsed. It used to take expanding the database -- whose
	// schema load was one of the few things that reported -- for the dot to catch
	// up with what the form had already been told.
	await expect(dot).toHaveAttribute('data-test-state', 'offline');

	// --- Reachable again ------------------------------------------------------

	await dsn.fill(seeded);
	await dsn.blur();
	await testConnection(page);
	await expect(page.getByText('Database connected')).toBeVisible();

	await expect(dot).toHaveAttribute('data-test-state', 'online');

	// --- Leaving it as it was found -------------------------------------------

	await expect(dsn).toHaveValue(seeded);
	await expect(treeNode(page, 'warehouse')).toBeVisible();
});

/**
 * The catalog rows under a database are not all alike. A schema, a "Tables"
 * group and a table all open on a click; a column has nothing to open, and that
 * click used to land on nothing at all. A row that does not expand now runs the
 * first entry of its own context menu, which for every one of them today is
 * "Infos...".
 *
 * Both columns on purpose: `email` is indexed, so it carries the index as a
 * child while still being a row nothing opens. What decides the click is
 * whether the row expands, not whether it holds anything.
 */
test('opens a row that does not expand from a single click', async ({ page, signIn }) => {
	await open(page, signIn);

	// Down to the columns of one table. Each of these rows expands, which is the
	// behaviour the new click must not have taken away.
	await treeNode(page, 'warehouse').click();
	await treeNode(page, 'main').click();
	await treeNode(page, 'Tables').click();
	await treeNode(page, 'customers').click();
	await treeNode(page, 'Columns').click();

	await expect(treeNode(page, 'country_code')).toBeVisible();
	await treeNode(page, 'country_code').click();

	await expect(modal(page)).toBeVisible();
	await expect(modal(page).getByText('country_code').first()).toBeVisible();

	// The backdrop, which is the only thing that dismisses this modal. The tree
	// is behind it, so the next row cannot be reached until it is gone.
	await page.mouse.click(5, 5);
	await expect(modal(page)).toBeHidden();

	await treeNode(page, 'email').click();

	await expect(modal(page)).toBeVisible();
	await expect(modal(page).getByText('email').first()).toBeVisible();
});
