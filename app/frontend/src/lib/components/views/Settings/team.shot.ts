import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing,
	type Page
} from '../../../../../tests/e2e/shots';

/**
 * The three lists a workspace is administered from, for the Users, Roles and
 * Groups pages.
 *
 * Each of those pages describes a screen it never shows. They are the same
 * screen in three tabs, so they are one spec: open settings once and
 * photograph each tab, rather than sign in three times to take three pictures
 * of the same window.
 *
 * The people, roles and groups are rows in the workspace database, seeded by
 * seedTeam in internal/cmd/e2eseed. Nobody in them is real.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/** Wide enough for the last column, short enough not to be mostly empty table. */
const FRAMING: Framing = { name: 'team', width: 1100, height: 420, density: 2 };

/**
 * A tab, and a row in it that only its own query could have produced. Waiting
 * on the tab's own name would pass the moment the button was clicked, with the
 * previous tab's table still on screen.
 */
const TABS = [
	{ id: 'Users', shot: 'users', row: 'priya@example.com' },
	{ id: 'Roles', shot: 'roles', row: 'data-engineer' },
	{ id: 'Groups', shot: 'groups', row: 'data-platform' }
] as const;

async function paintedAs(page: Page, theme: string) {
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
}

for (const theme of THEMES) {
	test.describe(`team ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('the users, roles and groups a workspace has', async ({ page, signIn }, info) => {
			const dir = shotsDirFor(info.file);

			await holdSession(page);
			await page.goto('/');
			await signIn();
			await paintedAs(page, theme);

			await page.getByRole('button', { name: 'Open Settings' }).click();

			for (const tab of TABS) {
				await page.getByRole('button', { name: tab.id, exact: true }).click();
				await expect(page.getByText(tab.row, { exact: true })).toBeVisible({ timeout: 15_000 });
				await shot(page, dir, `team.${tab.shot}.${theme}`, FRAMING);
			}
		});
	});
}
