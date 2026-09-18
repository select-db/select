import {
	shot,
	shotsEnabled,
	hideFileTreeChildren,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../app/frontend/tests/e2e/shots';
import { testId, editor } from '../../app/frontend/tests/e2e/selectors';

/**
 * The schema explorer, for the Schema Introspection page.
 *
 * The page spends most of its length on how the metadata is fetched and
 * cached, and one paragraph on the only part of it a reader ever sees: the
 * tree in the sidebar, opened down to a table's columns and its indexes.
 *
 * The tree is the picture, so the picture is the tree: the window is only as
 * wide as it needs to be for the panel beside it to look like an application
 * rather than a crop.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

const FRAMING: Framing = { name: 'schema', width: 1100, height: 900, density: 2 };

/** What the app stores when someone drags the sidebar wider, which is all this is. */
const SIDEBAR_WIDTH = 320;

for (const theme of THEMES) {
	test.describe(`schema ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('a table opened down to its columns and indexes', async ({ page, signIn }, info) => {
			await holdSession(page);
			await page.addInitScript((w) => {
				localStorage.setItem('leftbarWidth', String(w));
			}, SIDEBAR_WIDTH);
			await page.goto('/');
			await signIn();

			await page.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme);
			await expect
				.poll(() =>
					page.evaluate(() => {
						const [r, g, b] = (
							getComputedStyle(document.body).backgroundColor.match(/\d+/g) ?? ['255', '255', '255']
						).map(Number);
						return (r + g + b) / 3 < 128 ? 'dark' : 'light';
					})
				)
				.toBe(theme);

			// Each step waits for what the previous one produced: expanding a node
			// introspects the database, and how long that takes is not this file's
			// business. The categories SQLite reports and this database has nothing
			// in are hidden through the app's own visibility badge, or they are most
			// of the tree.
			const node = (name: string) => testId(page, 'tree.node', name);
			const appeared = (name: string) => expect(node(name)).toBeVisible({ timeout: 20_000 });

			// A file first, so the pane beside the tree is the editor rather than
			// the empty workspace's list of things to create. Nobody browses a
			// schema with nothing open.
			//
			// top_customers.sql rather than the query every other figure uses: the
			// hero types a predicate into weekly_revenue.sql and leaves it there
			// for its own next pass to clear, so in a full run this would be a
			// picture of half a WHERE clause and a lint warning about it.
			await node('top_customers.sql').click();
			await expect(editor.surface(page)).toBeVisible();

			await node('warehouse').click();
			await appeared('main');
			await hideFileTreeChildren(page, 0, ['sqlite_builtin']);

			await node('main').click();
			await appeared('Tables');
			// Indexes goes with them: this schema's only index belongs to `orders`,
			// and it reads better under the table than as a category of its own --
			// which is also the difference between the two nodes named Indexes.
			await hideFileTreeChildren(page, 1, [
				'Views',
				'Triggers',
				'Types',
				'Functions',
				'Pragmas',
				'Indexes'
			]);

			await node('Tables').click();
			await appeared('orders');

			await node('orders').click();
			// Triggers and Views stay: a table node carries no visibility badge, and
			// an empty category is what the app shows there anyway.
			await appeared('Columns');

			await node('Columns').click();
			await appeared('total_cents');

			await node('Indexes').click();
			await appeared('orders_customer_idx');

			await shot(page, shotsDirFor(info.file), `schema.${theme}`, FRAMING);
		});
	});
}
