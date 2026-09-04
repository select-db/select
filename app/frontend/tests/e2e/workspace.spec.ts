import { expect, holdSession, test } from './wails';
import { testId, editor } from './selectors';

/**
 * What the app does once it has a workspace to show. The state comes from
 * `internal/cmd/e2eseed`, which fills the run's data directory before the app
 * starts.
 */
test('shows the seeded workspace once signed in', async ({ page, signIn }) => {
	await page.goto('/');
	await signIn();

	await expect(page.getByText('weekly_revenue.sql')).toBeVisible();
});

/**
 * A query naming a variable nothing resolves stops for it, and the value typed
 * in reaches the database.
 *
 * The numeric type is the case worth a test: an input of type number binds a
 * number back, and the form used to call .trim() on it. That threw inside the
 * derivation behind the Run button, so an integer, decimal, date, timestamp or
 * time variable could be typed and never run.
 */
test('runs a query with a numeric runtime variable', async ({ page, signIn }) => {
	// Longer than half a second, which is when the app would otherwise decide it
	// has no token and drop back to the login screen.
	await holdSession(page);
	await page.goto('/');
	await signIn();

	await page.keyboard.press('ControlOrMeta+n');
	await expect(editor.surface(page)).toBeVisible();
	await editor.surface(page).click();
	await page.keyboard.type('SELECT id FROM orders WHERE total_cents >= $MIN_CENTS;');

	// A scratch tab starts attached to nothing, and the run is refused before it
	// ever looks for variables.
	await page.keyboard.press('ControlOrMeta+Shift+d');
	const picker = page.getByPlaceholder('Search db...');
	await expect(picker).toBeVisible();
	await page.getByText('warehouse', { exact: true }).last().click();
	await page.keyboard.press('Escape');
	await expect(picker).toBeHidden();

	await editor.surface(page).click();
	await page.keyboard.press('ControlOrMeta+Enter');

	const form = page.locator('form');
	await expect(page.getByText('Set variables')).toBeVisible();
	await form.locator('.select-container').first().click();
	await page.getByText('Integer', { exact: true }).last().click();
	await form.locator('input').click();
	await page.keyboard.type('5000');

	await form.getByRole('button', { name: 'Run' }).click();
	await expect(testId(page, 'segmented.option', 'results')).toHaveText(/[1-9]/, {
		timeout: 20_000
	});
});
