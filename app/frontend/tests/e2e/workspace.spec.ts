import { expect, test } from './wails';

/**
 * What the app does once it has a workspace to show. The state comes from
 * `internal/cmd/e2eseed`, which fills the run's data directory before the app
 * starts.
 */
test('shows the seeded workspace once signed in', async ({ page, signIn }) => {
	await page.goto('/');
	await signIn();

	await expect(page.getByText('queries')).toBeVisible();
});
