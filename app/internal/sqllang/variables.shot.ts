import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../frontend/tests/e2e/shots';
import { testId, editor } from '../../frontend/tests/e2e/selectors';

/**
 * The form SELECT puts up when a query names a variable nothing resolves, for
 * the Variables page.
 *
 * The page describes that form in a sentence and then spends a table on the
 * seven types a variable can be given. The types are the point -- they decide
 * whether the value is quoted -- and they are a control on screen, not prose.
 *
 * The query is typed into a scratch tab because a file with unresolved
 * variables in it would be a file every other capture has to run past.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

// Short: the figure is the form, and the app behind it is only there so the
// form has somewhere to be.
const FRAMING: Framing = { name: 'variables', width: 1000, height: 440, density: 2 };

/** Two variables, so the form has more than one row and the types differ. */
const QUERY = 'SELECT id, status, total_cents FROM orders\nWHERE status = $STATUS AND total_cents >= $MIN_CENTS;';

for (const theme of THEMES) {
	test.describe(`variables ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('a query stopped for the values it is missing', async ({ page, signIn }, info) => {
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

			await page.keyboard.press('ControlOrMeta+n');
			await expect(editor.surface(page)).toBeVisible();
			await editor.surface(page).click();
			await page.keyboard.type(QUERY);

			// A scratch tab starts attached to nothing, and the run is refused
			// before it ever looks for variables.
			await page.keyboard.press('ControlOrMeta+Shift+d');
			const picker = page.getByPlaceholder('Search db...');
			await expect(picker).toBeVisible({ timeout: 10_000 });
			await page.getByText('warehouse', { exact: true }).last().click();
			await page.keyboard.press('Escape');
			await expect(picker).toBeHidden();

			await editor.surface(page).click();
			await page.keyboard.press('ControlOrMeta+Enter');

			// The form, one row per name nothing resolved.
			await expect(page.getByText('Set variables')).toBeVisible({ timeout: 20_000 });
			const row = (name: string) => page.locator('.row').filter({ hasText: `$${name}` });

			// Typed rather than filled: the form reads its values from the input
			// events, and a set-and-fire fill leaves Run disabled.
			await row('STATUS').locator('input').click();
			await page.keyboard.type('paid');

			// The second one is a number, which is the whole reason the type is a
			// control: text would reach the database quoted.
			await row('MIN_CENTS').locator('.select-container').first().click();
			await page.getByText('Integer', { exact: true }).last().click();
			await row('MIN_CENTS').locator('input').click();
			await page.keyboard.type('5000');

			await shot(page, shotsDirFor(info.file), `variables.${theme}`, FRAMING);

			// And the values published above are ones the query actually runs with.
			// Asserted after the shutter because it is the figure that has to be of
			// the form, not of what comes back.
			// The form's Run, not the toolbar's: the query behind it has one too.
			await page.locator('form').getByRole('button', { name: 'Run' }).click();
			await expect(testId(page, 'segmented.option', 'results')).toHaveText(/[1-9]/, {
				timeout: 20_000
			});
		});
	});
}
