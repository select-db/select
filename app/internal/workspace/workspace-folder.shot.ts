import { mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';

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
} from '../../frontend/tests/e2e/shots';
import { call, intercept } from '../../frontend/tests/e2e/wails';
import { testId } from '../../frontend/tests/e2e/selectors';

/**
 * The two screens "Opening a folder" is about: the one a signed-in user lands
 * on with no folder open, and the one a folder that is not a workspace yet
 * opens as.
 *
 * Two of the four outcomes, not four pictures: these are the two a person does
 * something on. The other two -- a folder from another server, a workspace they
 * are not in -- are the same screen carrying a different sentence, and the
 * sentence is already in the page.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/** Wide enough for the left bar and the panel beside it, at the size the docs show. */
const FRAMING: Framing = { name: 'folder', width: 820, height: 340, density: 2 };

const WORKSPACE = 'selectDb/internal/workspace.Workspace';

/** PickFolder, from the generated bindings. An OS dialog no browser can drive. */
const PICK_FOLDER = 2753062023;

/** The folder the setup figure opens. Taken away again in the teardown below. */
const DEMO_FOLDER = '/tmp/code/reports';

/** ListFolders, from the generated bindings. */
const LIST_FOLDERS = 3511278583;

/**
 * What the start figure shows under "Recent".
 *
 * Answered rather than seeded: the real list is one folder, at the worker's
 * temp path, so the picture would be a single row reading
 * `/var/folders/../T/select-e2e/0/workspace`. That says nothing about opening a
 * folder and everything about this suite. These are the shape the list really
 * has -- name and path, current one first -- with paths a reader recognises.
 */
const RECENT = [
	{ name: 'analytics', path: '/Users/sam/code/analytics' },
	{ name: 'billing', path: '/Users/sam/code/billing' },
	{ name: 'growth-reports', path: '/Users/sam/work/growth-reports' }
];

/** Clears any toast, which belongs to how a state was reached, not to the state. */
async function dismissToasts(page: Page) {
	const toasts = page.locator('.alert-wrapper');
	for (let open = await toasts.count(); open > 0; open = await toasts.count()) {
		// Two icon buttons, copy and close, neither with a name to ask for.
		await toasts.first().locator('button').last().click();
		await expect(toasts).toHaveCount(open - 1);
	}
}

// Every capture drives the one workspace this worker has, so a spec that leaves
// it closed hands the next one an empty workbench.
test.afterEach(async ({ request, dataDir }) => {
	await call(request, `${WORKSPACE}.OpenFolder`, join(dataDir, 'workspace'));
	rmSync(DEMO_FOLDER, { recursive: true, force: true });
});

for (const theme of THEMES) {
	test.describe(`workspace folder ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('the start screen, and a folder that is not a workspace yet', async ({
			page,
			signIn,
			request,
			dataDir
		}, info) => {
			const dir = shotsDirFor(info.file);

			await holdSession(page);

			await intercept(page, LIST_FOLDERS, (route) =>
				route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify(RECENT)
				})
			);

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

			// Nothing is open on either screen, so the file tree has nothing to
			// show: closed, each picture is the screen it is about. Through the
			// bottom bar's own button, which is only there while a workspace is,
			// so it has to happen before the folder closes.
			await page.getByRole('button', { name: 'Files' }).click();
			await expect(testId(page, 'tree.panel')).toBeHidden();

			// 1. Signed in, nothing open: the folders this machine has, where the
			// files would be.
			await call(request, `${WORKSPACE}.CloseFolder`);
			await expect(testId(page, 'folder.screen', 'no_folder')).toBeVisible();

			// Closing a folder out from under the app is how this state is reached
			// here, and it says so in a toast. Signing in with nothing open is the
			// state the page is about, and that arrives without one.
			await dismissToasts(page);
			await shot(page, dir, `folder.start.${theme}`, FRAMING);

			// 2. A folder with no config in it. Answered rather than clicked: the
			// picker is an OS dialog, and `Cmd+O` is the gesture the page names.
			// Outside the worker's data directory, and named, because this path is
			// in the picture: under dataDir it reads
			// `/var/folders/../T/select-e2e/0/reports`, which tells a reader about
			// this suite rather than about opening a folder. Shots run on one
			// worker, so a fixed path cannot collide with another.
			mkdirSync(DEMO_FOLDER, { recursive: true });
			await intercept(page, PICK_FOLDER, (route) =>
				route.fulfill({ status: 200, body: DEMO_FOLDER })
			);
			await page.keyboard.press('ControlOrMeta+o');

			await expect(testId(page, 'folder.screen', 'needs_setup')).toBeVisible({ timeout: 15_000 });
			// The name is filled in from the folder, which is half of what this
			// figure is for: photographed empty it would say the opposite.
			await expect(page.getByPlaceholder('Workspace name')).not.toHaveValue('');
			await shot(page, dir, `folder.setup.${theme}`, FRAMING);

			// Left unmade on purpose: creating the workspace here would add a
			// second folder to every picker photographed after this one.
		});
	});
}
