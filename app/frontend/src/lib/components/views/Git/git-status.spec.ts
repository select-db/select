import { expect, exec, open, test, workspaceId } from '../../../../../tests/e2e/wails';
import { testId, treeRow } from '../../../../../tests/e2e/selectors';
import { choose, openRootMenu } from '../../../../../tests/e2e/tree';

/**
 * A database is a directory, and a brand new one is a directory git has never
 * seen. `git status --porcelain` collapses an untracked directory into a single
 * entry for the directory, so everything a new database is made of -- its
 * db.config.json first of all -- was reported as "mydb/" and then dropped,
 * leaving the panel to say the workspace had not changed.
 *
 * The config is also a row in the file tree now: it is a file people read, edit
 * and commit, so both views are checked here from the one database.
 */
test('a new database shows its config in the tree and in the git panel', async ({
	page,
	request,
	signIn
}) => {
	await open(page, signIn);
	const id = await workspaceId(request);

	// Made through the app, so the config carries an id of its own rather than
	// a copy of another database's.
	await openRootMenu(page);
	await choose(page, 'New Database...');

	// --- The tree ------------------------------------------------------------

	await expect(treeRow(page, 'db #1')).toBeVisible();
	await treeRow(page, 'db #1').click();
	await expect(treeRow(page, 'db.config.json')).toBeVisible();

	// The workspace's own config is a row too: a teammate clones it to land in
	// the same workspace, so it is read and committed like any other file.
	await expect(treeRow(page, 'select.config.json')).toBeVisible();

	// --- The git panel -------------------------------------------------------

	await page.getByRole('button', { name: 'Source control' }).click();
	await expect(testId(page, 'git.panel')).toBeVisible();

	// Named by its path, because the panel lists a file per row and two
	// databases both have a db.config.json.
	const listed = () =>
		testId(page, 'git.panel')
			.locator('[data-test="tree.node"]')
			.filter({ hasText: 'db.config.json' })
			.count();
	await expect.poll(listed, { timeout: 15_000 }).toBeGreaterThan(0);

	// --- Leaving it as it was found ------------------------------------------

	await exec(request, id, 'rm', '-rf', 'db #1');
	await expect(treeRow(page, 'db #1')).toHaveCount(0);
});
