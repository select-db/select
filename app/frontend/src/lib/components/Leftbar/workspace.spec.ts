import { join } from 'node:path';

import {
	call,
	expect,
	holdSession,
	intercept,
	test,
	type APIRequestContext,
	type Page
} from '../../../../tests/e2e/wails';

/**
 * The Open folder button the leftbar falls back to when this machine has no
 * workspace folder yet. It is the first thing a new user clicks and the only way
 * in from that state, so it has to survive being cancelled.
 *
 * The picker is an OS dialog no browser can drive, so this answers PickFolder
 * and counts the asks: whether the app asked for the dialog is all there is to
 * observe from outside it.
 */

const WORKSPACE = 'selectDb/internal/workspace.Workspace';
const PICK_FOLDER = 2753062023;
const LIST_FOLDERS = 3511278583;

/** The folder the seed left, which the app reopens on login. */
const seeded = (dataDir: string) => join(dataDir, 'workspace');

/**
 * Signs in with nothing remembered and no folder open, which is the whole of
 * what the fallback button is for. `asked` counts the pickers cancelled since.
 */
async function readyToPick(page: Page, request: APIRequestContext, signIn: () => Promise<void>) {
	const asked = { count: 0 };

	await holdSession(page);

	// An empty list is what a machine that has never opened a folder has, and it
	// is what leaves the leftbar on the button rather than on the picker menu.
	await intercept(page, LIST_FOLDERS, (route) =>
		route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
	);

	await intercept(page, PICK_FOLDER, async (route) => {
		asked.count += 1;
		// "" is what PickFolder returns when the dialog is cancelled.
		await route.fulfill({ status: 200, body: '' });
	});

	await page.goto('/');
	await signIn();
	await call(request, `${WORKSPACE}.CloseFolder`);

	const button = page.getByRole('button', { name: 'Open folder' });
	await expect(button).toBeVisible();

	return { asked, button };
}

// Specs share one app per worker, so a spec that leaves no folder open would
// hand the next one a workbench it never asked for.
test.afterEach(async ({ request, dataDir }) => {
	await call(request, `${WORKSPACE}.OpenFolder`, seeded(dataDir));
});

test('the picker can be opened again after it is cancelled', async ({ page, request, signIn }) => {
	const { asked, button } = await readyToPick(page, request, signIn);

	await button.click();
	await expect.poll(() => asked.count).toBe(1);

	// Cancelling opened no folder, so the same button is still the only way in.
	await button.click();
	await expect.poll(() => asked.count).toBe(2);
});
