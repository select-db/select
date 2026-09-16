import {
	databasesInGraph,
	onDisk,
	expect,
	exec,
	open,
	test,
	workspaceId,
	type Page
} from '../../../../../tests/e2e/wails';
import { dbStatus, renameBox, tab, testId, treeRow } from '../../../../../tests/e2e/selectors';
import { choose, openRowMenu, openRootMenu, renameTo } from '../../../../../tests/e2e/tree';

/**
 * A database is a directory named after itself. Its name is written down
 * nowhere else, so everything that changes the name changes the directory and
 * everything that changes the directory changes the name.
 *
 * The first scenario is about that one fact, in one sequence rather than a test
 * per gesture for the same reason as filesystem.spec.ts: each step is only
 * meaningful on the state the last one left. The second is about the other
 * thing a row says, whether the database answers.
 *
 * Both leave the seeded workspace as they found it.
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
	await open(page, signIn);

	const id = await workspaceId(request);
	await expect(treeRow(page, 'warehouse')).toBeVisible();

	// --- Made ---------------------------------------------------------------

	// A new database is a directory in the workspace root, named for the
	// database rather than for its id.
	await openRootMenu(page);
	await choose(page, 'New Database...');
	await expect(treeRow(page, 'db #1')).toBeVisible();
	expect(await onDisk(request, id, 'db #1')).toBe(true);

	// --- Renamed from the tree ----------------------------------------------

	await openRowMenu(page, 'db #1');
	await choose(page, 'Rename...');
	await expect(renameBox(page)).toBeFocused();

	// A database's config is written while its form is open -- the form saves on
	// a debounce -- and that arrives as a db_instance update like any other. It
	// used to close the rename box, so whatever was being typed went nowhere.
	// Touching the config raises the same update without the wait.
	await exec(request, id, 'touch', 'db #1/db.config.json');
	await expect(renameBox(page)).toBeFocused();

	await renameTo(page, 'analytics');

	await expect(treeRow(page, 'analytics')).toBeVisible();
	await expect(treeRow(page, 'db #1')).toHaveCount(0);

	// The directory went with it, both ways: the new name is there and the old
	// one is not left behind.
	expect(await onDisk(request, id, 'analytics')).toBe(true);
	expect(await onDisk(request, id, 'db #1')).toBe(false);
	expect(await databasesInGraph(request)).toContain('analytics');

	// --- What a name is allowed to be ---------------------------------------

	// A database renames through the same call a folder does, so a name already
	// in the folder is refused on the same terms. The row goes back to saying
	// what it is still called.
	await openRowMenu(page, 'analytics');
	await choose(page, 'Rename...');

	// Typed here rather than through renameTo, so a config write can land between
	// the typing and the Enter that commits it. The row re-renders, and the guard
	// that swallows the Enter opening the box used to be re-armed by that
	// re-render -- so the Enter below went nowhere and the box could no longer be
	// committed or closed from the keyboard.
	const box = renameBox(page);
	await expect(box).toBeFocused();
	await box.press('ControlOrMeta+a');
	await box.fill('warehouse');
	await exec(request, id, 'mv', 'warehouse', 'depot');
	await expect(treeRow(page, 'depot')).toBeVisible();
	await exec(request, id, 'mv', 'depot', 'warehouse');
	await expect(treeRow(page, 'warehouse')).toBeVisible();
	await box.press('Enter');
	await expect(box).toBeHidden();

	await expect(treeRow(page, 'analytics')).toBeVisible();
	expect(await databasesInGraph(request)).toEqual(['analytics', 'warehouse']);

	// --- Renamed again, and the form follows --------------------------------

	// The tree is the one way to rename, and the form open on the database takes
	// the new name with it rather than holding the old one.
	await openRowMenu(page, 'analytics');
	await choose(page, 'Edit...');
	await expect(tab(page, 'analytics')).toBeVisible();

	await openRowMenu(page, 'analytics');
	await choose(page, 'Rename...');
	await renameTo(page, 'reporting');

	await expect(treeRow(page, 'reporting')).toBeVisible();
	await expect(treeRow(page, 'analytics')).toHaveCount(0);
	await expect(tab(page, 'reporting')).toBeVisible();
	expect(await onDisk(request, id, 'reporting')).toBe(true);

	// --- Moved --------------------------------------------------------------

	// The directory is a directory: moving it into a folder moves the database,
	// which keeps its name because the move did not change it.
	await openRootMenu(page);
	await choose(page, 'New folder...');
	await renameTo(page, 'europe');
	await expect(treeRow(page, 'europe')).toBeVisible();

	await exec(request, id, 'mv', 'reporting', 'europe/reporting');

	await expect(treeRow(page, 'europe')).toBeVisible();
	await treeRow(page, 'europe').click();
	await expect(treeRow(page, 'reporting')).toBeVisible();
	expect(await onDisk(request, id, 'reporting')).toBe(false);
	expect(await databasesInGraph(request)).toContain('reporting');

	// --- Renamed from outside the app ---------------------------------------

	// Nothing tells the app; it finds out by watching, and the name it shows is
	// the directory's whoever changed it.
	await exec(request, id, 'mv', 'europe/reporting', 'europe/eu-metrics');
	await expect(treeRow(page, 'eu-metrics')).toBeVisible();
	await expect(treeRow(page, 'reporting')).toHaveCount(0);

	// And the watch went with it: a file made inside the renamed directory
	// still reaches the tree, which it does not if the watch is left pointing
	// at the name the directory had.
	await exec(request, id, 'touch', 'europe/eu-metrics/inside.sql');
	await treeRow(page, 'eu-metrics').click();
	await expect(treeRow(page, 'inside.sql')).toBeVisible();

	// --- Deleted ------------------------------------------------------------

	await openRowMenu(page, 'eu-metrics');
	await choose(page, 'Delete');
	await expect(treeRow(page, 'eu-metrics')).toHaveCount(0);
	await expect(tab(page, 'eu-metrics')).toHaveCount(0);

	// --- Leaving it as it was found -----------------------------------------

	await openRowMenu(page, 'europe');
	await choose(page, 'Delete');
	await expect(treeRow(page, 'europe')).toHaveCount(0);

	expect(await databasesInGraph(request)).toEqual(['warehouse']);
	await expect(treeRow(page, 'warehouse')).toBeVisible();
});

test('shows what the last attempt to reach a database found', async ({ page, signIn }) => {
	await open(page, signIn);

	await expect(treeRow(page, 'warehouse')).toBeVisible();

	// The availability watcher pings on its own every few seconds, so the dot
	// arrives at "online" without anyone asking. That is the state the rest of
	// this test moves away from and back to.
	// Scoped to the tree: the same indicator appears wherever a database is named
	// -- an open tab, a results badge -- and the tree is where it went stale.
	const dot = testId(page, 'tree.panel').locator(dbStatus(page, WAREHOUSE));
	await expect(dot).toHaveAttribute('data-test-state', 'online', { timeout: 30_000 });

	await openRowMenu(page, 'warehouse');
	await choose(page, 'Edit...');

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
	await expect(treeRow(page, 'warehouse')).toBeVisible();
});
