import {
	shot,
	fillLabelled,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing,
	type Page
} from '../../../../../tests/e2e/shots';
import { testId } from '../../../../../tests/e2e/selectors';
import { choose, openRowMenu, renameTo } from '../../../../../tests/e2e/tree';

/**
 * The connection form in its three shapes, for Connecting a Database, SSH
 * Tunnels and Proxified Connections.
 *
 * `db_type !== 'sqlite'` gates the proxy, tunnel and pool blocks; the proxy
 * checkbox gates pool tuning; the mode tab swaps the DSN for a tunnel; the auth
 * method swaps the tunnel's fields.
 *
 * Creates its own Postgres datasource and deletes it again: `warehouse` cannot
 * be borrowed because the form autosaves 600ms after any change, which would
 * leave the sample workspace on the wrong dialect for every later shot.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

const FRAMING: Framing = { name: 'dbform', width: 940, height: 640, density: 2 };

/**
 * Proxifying adds a host key, an authentication method and four pool settings
 * to a form that already filled the window, so that figure gets a window tall
 * enough to hold the whole thing. Scrolling instead would put the checkbox that
 * causes all of it out of frame, and the pool settings are what the section
 * beside this picture is about.
 */
const TALL: Framing = { ...FRAMING, height: 940 };

const SAMPLE_DB = 'warehouse';

/** The database this spec makes to photograph, and takes away again. */
const CREATED = 'analytics-prod';

/**
 * What the app calls a database it has just made. Cleared alongside CREATED so
 * the number is always 1: a pass that died between creating and renaming would
 * otherwise leave one behind and push the next one to `db #2`.
 */
const NEW_DB = 'db #1';

/**
 * Deletes every database in the tree called name, and waits for the tree to
 * agree that they are gone.
 *
 * A loop rather than one delete: two passes run against one app, so a pass that
 * failed before its own teardown leaves a database behind, and the next pass
 * would otherwise photograph a workspace with two of them in it.
 */
async function removeDatasources(page: Page, name: string) {
	const nodes = testId(page, 'tree.node', name);
	const confirm = page.getByRole('button', { name: 'Delete and revoke' });

	// Retried rather than waited on: a right-click on a row the tree is still
	// moving opens the menu on whatever arrives under the pointer, and the
	// database stays.
	await expect
		.poll(
			async () => {
				if ((await nodes.count()) === 0) return 0;

				await nodes.first().click({ button: 'right' });
				await choose(page, 'Delete');

				// A proxified database keeps its credential on the server, so
				// deleting it asks before dropping that for the whole workspace. A
				// local one is never asked about, so the dialog is waited for only
				// long enough to tell the two apart.
				await confirm.waitFor({ state: 'visible', timeout: 5_000 }).catch(() => {});
				if (await confirm.isVisible()) await confirm.click();

				// The row goes when the graph comes back, not when the click
				// returns. Reporting before that sends the next attempt at a row on
				// its way out, whose menu belongs to nothing by the time it opens.
				await expect(nodes)
					.toHaveCount(0, { timeout: 15_000 })
					.catch(() => {});
				return nodes.count();
			},
			{ timeout: 30_000 }
		)
		.toBe(0);
}

/**
 * Closes any connection error the form has raised.
 *
 * The DSN here points at nothing -- it is a form being photographed, not a
 * database being used -- so the app reports that it cannot reach it, correctly
 * and in a toast across the bottom of the window. That belongs in a screenshot
 * about a failed connection, not in one about the fields above it.
 */
async function dismissErrors(page: Page) {
	const alerts = page.locator('.alert-wrapper');
	for (let open = await alerts.count(); open > 0; open = await alerts.count()) {
		// Two icon buttons, copy and close, neither with a name to ask for.
		await alerts.first().locator('button').last().click();
		await expect(alerts).toHaveCount(open - 1);
	}
}

for (const theme of THEMES) {
	test.describe(`db form ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('local, tunnelled and proxified', async ({ page, signIn }, info) => {
			// Three captures, two viewport changes and a database made and taken
			// away again, against the default 30s for a test that usually does one
			// of those. The teardown was what ran out of budget, which leaves the
			// database behind for every spec after this one.
			test.slow();

			const dir = shotsDirFor(info.file);

			await holdSession(page);
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

			// Count what is in the tree before, so the cleanup at the end has
			// something to prove itself against.
			const datasources = testId(page, 'tree.node', SAMPLE_DB);
			await expect(datasources).toHaveCount(1);

			// The light pass runs first and removes what it made, but a pass that
			// failed part-way through did not. Start from the workspace this spec
			// expects rather than from whatever the last run left in it.
			await removeDatasources(page, CREATED);
			await removeDatasources(page, NEW_DB);

			// A database of our own, through the same action a person uses. The
			// root menu hangs off the file panel itself, so the right-click has to
			// land on empty space below the tree rather than on an item in it,
			// which carries its own menu.
			const panel = page.getByRole('region', { name: 'File system root drop zone' });
			const area = await panel.boundingBox();
			if (!area) throw new Error('no file panel to right-click');
			await page.mouse.click(60, area.y + area.height - 10, { button: 'right' });
			await page.getByText('New Database...', { exact: true }).click();
			await expect(testId(page, 'datasource.form')).toBeVisible();

			const dsn = testId(page, 'datasource.dsn').locator('input');

			// A name worth photographing. The app numbers a new database `db #1`,
			// which tells a reader nothing about what they are looking at.
			//
			// Renamed from the tree: a database is named by its folder, and the
			// form derives what it shows from the URI rather than offering a field
			// for it.
			await openRowMenu(page, NEW_DB);
			await choose(page, 'Rename...');
			await renameTo(page, CREATED);
			await expect(testId(page, 'tree.node', CREATED)).toBeVisible({ timeout: 15_000 });

			// 1. Postgres, local. What a networked database opens as, and the form
			// the Connecting page is written about: the proxy, connection-mode and
			// DSN blocks that the SQLite form does not have at all.
			// A new database opens as PostgreSQL already, which is the networked
			// form this figure is for; nothing has to touch the dialect picker.
			await expect(page.getByText('PostgreSQL', { exact: true }).first()).toBeVisible();
			await dsn.fill('host=$PG_HOST port=5432 user=$PG_USER password=$PG_PASS dbname=analytics');
			await expect(page.getByText('Proxy connection')).toBeVisible();

			await dismissErrors(page);
			await shot(page, dir, `dbform.local.${theme}`, FRAMING);

			// 2. Through an SSH tunnel. The mode tab adds the tunnel's own host,
			// port, user and authentication above the DSN.
			//
			// The fields are filled rather than left on their placeholders: an
			// empty required field is a red validation message, and a figure
			// covered in them says the form is broken rather than showing what it
			// asks for.
			await page.getByText('DSN + SSH tunnel', { exact: true }).click();
			await fillLabelled(page, 'SSH user', 'deploy');
			await fillLabelled(page, 'SSH tunnel host', 'bastion.internal');
			// A fresh non-proxified tunnel defaults to the SSH agent, which is the
			// method worth showing anyway: no secret to type into the form or keep
			// anywhere after it.
			await expect(page.getByText('SSH agent', { exact: true }).first()).toBeVisible();
			await dismissErrors(page);
			await shot(page, dir, `dbform.ssh.${theme}`, FRAMING);

			// 3. Proxified: credentials held server-side rather than on the
			// machine, and the pool tuning that only exists in that mode.
			await page.getByText('Proxified', { exact: true }).click();
			await expect(page.getByText('Max open connections')).toBeVisible({ timeout: 15_000 });
			// Proxifying makes the host key mandatory, because the server is the
			// one making the connection and has no user to ask about a fingerprint.
			// ssh-keyscan output, which is the shape the field validates against:
			// hostname, key type, then the key.
			await fillLabelled(
				page,
				'SSH host key',
				'bastion.internal ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB6mQ2Xk'
			);
			// The authentication method is deliberately left unchosen. Proxifying
			// clears it -- the agent it defaulted to lives on a laptop the server
			// cannot reach -- and every remaining method wants a secret, which
			// would put an empty required field in a figure about pooling.
			await page.setViewportSize({ width: TALL.width, height: TALL.height });
			await expect(page.getByText('Max idle time (s, 0 = no limit)')).toBeVisible();
			await dismissErrors(page);
			await shot(page, dir, `dbform.proxified.${theme}`, TALL);
			await page.setViewportSize({ width: FRAMING.width, height: FRAMING.height });

			// Put the workspace back. Every capture drives this one workspace, so
			// a database left behind is a database in every screenshot after it.
			await removeDatasources(page, CREATED);
			await expect(datasources).toHaveCount(1);
		});
	});
}
