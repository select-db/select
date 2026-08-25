import { call, expect, test } from './wails';

/**
 * The first things worth knowing about a build: it starts, it paints, and the
 * frontend and the Go side can talk to each other in both directions.
 */
test.beforeEach(async ({ page }) => {
	await page.goto('/');
});

test('boots to its first screen without console errors', async ({ page, consoleErrors }) => {
	await expect(page.getByText('Log in with Github')).toBeVisible();
	expect(consoleErrors).toEqual([]);
});

test('calls a bound Go method and gets its value back', async ({ request }) => {
	// The env is stamped in at build time (-X main.appEnv) and read back out of
	// the process env by the service, so a correct answer means the whole chain
	// is wired: build flags, service registration, bindings, transport.
	await expect(call(request, 'selectDb/internal/system.System.GetAppEnv')).resolves.toBe('dev');
});

test('applies a theme the backend broadcasts', async ({ page, emit }) => {
	// themeStore listens for this from module scope, so it is live on the login
	// screen, and what it does with the payload — writing custom properties onto
	// the document root — is visible from here.
	await emit('themeUpdated', { shared: { '--e2e-probe': 'rgb(1, 2, 3)' }, light: {}, dark: {} });

	await expect
		.poll(() =>
			page.evaluate(() => document.documentElement.style.getPropertyValue('--e2e-probe').trim())
		)
		.toBe('rgb(1, 2, 3)');
});
