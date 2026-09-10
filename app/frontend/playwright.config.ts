import { defineConfig } from '@playwright/test';

/**
 * Drives the real application built with Wails' `server` tag — the same Go
 * services, bindings and events as the desktop build, with the webview replaced
 * by an HTTP server, so an ordinary browser can drive it and CI needs no
 * display. Run it through `wails3 task test:e2e`, which builds that binary
 * first.
 *
 * It cannot cover what needs a native window — zoom, dialogs, menus — and the
 * engine under test is Chromium, not WebKit or WebView2.
 *
 * Each worker seeds a workspace and starts its own app on its own port, in the
 * `app` fixture in `tests/e2e/app.ts`, which is also where `baseURL` comes
 * from. There is no `webServer` here for that reason: one shared server is
 * exactly what a second worker cannot have.
 */

export default defineConfig({
	testDir: 'tests/e2e',

	/**
	 * `e2e` is the suite: specs under tests/e2e, run by `wails3 task test:e2e`
	 * and by CI.
	 *
	 * `shots` writes the website's product screenshots. Each spec sits beside the
	 * content that shows it. They write into the repo, so CI never selects this
	 * project; `wails3 task shots` does, and sets SHOTS=1.
	 */
	projects: [
		{ name: 'e2e', testDir: 'tests/e2e', testMatch: /\.spec\.ts$/ },
		{
			name: 'shots',
			testDir: '../..',
			testMatch: /\.shot\.ts$/,
			testIgnore: ['**/node_modules/**', '**/build/**', '**/dist/**', '**/.svelte-kit/**']
		}
	],

	forbidOnly: !!process.env.CI,

	/**
	 * Files spread across workers; the tests inside one do not. A spec is a
	 * sequence of gestures on the workspace its worker owns, and reordering them
	 * is not a thing any of them survive.
	 *
	 * `shots` writes images into the repo and stays serial regardless.
	 */
	fullyParallel: false,
	workers: process.env.SHOTS ? 1 : undefined,

	/** No retries: a flake stays visible instead of being retried away. */
	retries: 0,
	reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],

	use: {
		trace: 'retain-on-failure'
	}
});
