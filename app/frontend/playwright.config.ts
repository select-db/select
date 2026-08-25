import { defineConfig } from '@playwright/test';
import { execFileSync } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

/**
 * Drives the real application built with Wails' `server` tag — the same Go
 * services, bindings and events as the desktop build, with the webview replaced
 * by an HTTP server, so an ordinary browser can drive it and CI needs no
 * display. Run it through `wails3 task test:e2e`, which builds that binary
 * first.
 *
 * It cannot cover what needs a native window — zoom, dialogs, menus — and the
 * engine under test is Chromium, not WebKit or WebView2.
 */

const PORT = Number(process.env.E2E_PORT ?? 9346);
const HOST = '127.0.0.1';

// The app keeps its SQLite DB and user config under the OS config dir. Point it
// at a throwaway one so a test run never reads or writes the developer's real
// workspaces. `XDG_CONFIG_HOME` covers Linux, `HOME` covers macOS.
const dataDir = mkdtempSync(join(tmpdir(), 'select-e2e-'));

// Fill it before the app starts: a migrated database, a user, the workspace
// they are in, and its files — without which the app only ever shows a login
// screen. See `internal/cmd/e2eseed`. This runs here rather than in a
// globalSetup because Playwright starts `webServer` first, and the app reads
// all of this while booting.
// `-tags server` for the same reason the binary under test uses it: without it
// the build pulls in wails' GUI cgo path and needs GTK headers no headless
// runner has.
execFileSync('go', ['run', '-tags', 'server', './internal/cmd/e2eseed', dataDir], {
	cwd: '..',
	env: { ...process.env, CGO_ENABLED: '0' },
	stdio: 'inherit'
});

export default defineConfig({
	testDir: 'tests/e2e',
	forbidOnly: !!process.env.CI,
	// One worker, no retries: every page talks to the same app process, and an
	// event emitted by one test is broadcast to all of them. Serial keeps that
	// honest, and a flake stays visible instead of being retried away.
	workers: 1,
	reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : [['list']],

	use: {
		baseURL: `http://${HOST}:${PORT}`,
		trace: 'retain-on-failure'
	},

	webServer: {
		command: `../build/bin/select-server`,
		url: `http://${HOST}:${PORT}/`,
		reuseExistingServer: !process.env.CI,
		stdout: 'pipe',
		stderr: 'pipe',
		timeout: 60_000,
		env: {
			WAILS_SERVER_HOST: HOST,
			WAILS_SERVER_PORT: String(PORT),
			XDG_CONFIG_HOME: dataDir,
			HOME: dataDir
		}
	}
});
