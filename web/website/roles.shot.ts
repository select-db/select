import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../app/frontend/tests/e2e/shots';

/**
 * The picture beside "Stop asking someone for prod": a role that can read the
 * warehouse and cannot read two of its columns.
 *
 * The rules are real rows in the workspace database, seeded by seedRoles in
 * internal/cmd/e2eseed. Column-level grants are the claim this row makes, so
 * the fixture carries them rather than the picture implying them.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

// Wide enough for the last action column. At 940 the results table was cut after
// UPDATE, and a table sliced down the middle reads as a broken picture
// whatever it is showing.
const FRAMING: Framing = { name: 'roles', width: 1180, height: 620, density: 2 };

const ROLE = 'analyst-readonly';
const DENIED = 'email';

for (const theme of THEMES) {
	test.describe(`roles ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('a role that cannot read one column', async ({ page, signIn }, info) => {
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

			await page.getByRole('button', { name: 'Open Settings' }).click();
			await page.getByRole('button', { name: 'Roles', exact: true }).click();

			await expect(page.getByText(ROLE).first()).toBeVisible({ timeout: 15_000 });
			await page.getByText(ROLE).first().click();

			// Down to the column. The results table opens at the database, and a row of
			// databases says nothing that "we have roles" does not: the claim this
			// picture makes is that a grant can be one column wide.
			for (const level of ['warehouse', 'main', 'customers']) {
				await page.getByRole('button', { name: level, exact: true }).first().click();
			}
			await expect(page.getByText(DENIED, { exact: true }).first()).toBeVisible({
				timeout: 15_000
			});

			// The results table opens on the workspace-level rules, which are the dullest
			// thing in it. Scroll the columns into frame: the denial next to its
			// granted siblings is what this picture is for.
			const lastColumn = page.getByText('created_at', { exact: true }).first();
			await lastColumn.scrollIntoViewIfNeeded();
			await expect(lastColumn).toBeVisible();

			await shot(page, shotsDirFor(info.file), `roles.${theme}`, FRAMING);
		});
	});
}
