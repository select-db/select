import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs';
import { basename, join } from 'node:path';

import {
	call,
	expect,
	holdSession,
	intercept,
	test,
	type APIRequestContext,
	type Page
} from '../../../../tests/e2e/wails';
import { testId, treeRow } from '../../../../tests/e2e/selectors';

/**
 * The screens between signing in and looking at files, one per thing opening a
 * folder can find.
 *
 * The picker is the one step no browser can drive: it is an OS dialog. Every
 * spec here answers PickFolder with the folder it means and then clicks the
 * button a person clicks, so everything after the dialog is the app's own.
 */

const WORKSPACE = 'selectDb/internal/workspace.Workspace';
const PICK_FOLDER = 2753062023;

const screen = (page: Page) => testId(page, 'folder.screen');

/** The folder the seed left, which the app reopens on login. */
const seeded = (dataDir: string) => join(dataDir, 'workspace');

/** A folder nothing has been done to yet. */
const scratch = (dataDir: string, prefix: string) => mkdtempSync(join(dataDir, prefix));

/** The server the fixture signed the user in to, as its own config records it. */
function currentServer(dataDir: string) {
	const config = readFileSync(join(seeded(dataDir), 'select.config.json'), 'utf8');
	return JSON.parse(config).server as string;
}

/** Answers the OS folder dialog with a folder of the spec's choosing. */
async function answerPicker(page: Page, folder: string) {
	await intercept(page, PICK_FOLDER, (route) => route.fulfill({ status: 200, body: folder }));
}

/**
 * Signs in, closes the seeded folder, and answers the picker with `folder`:
 * the state a person is in when they click Open folder.
 */
async function readyToOpen(
	page: Page,
	request: APIRequestContext,
	signIn: () => Promise<void>,
	folder: string
) {
	await holdSession(page);
	await page.goto('/');
	await signIn();
	await answerPicker(page, folder);
	await call(request, `${WORKSPACE}.CloseFolder`);
	await expect(screen(page)).toHaveAttribute('data-test-value', 'no_folder');
	await openFolderButton(page).click();
}

/** The one button that opens a folder, in the corner of the leftbar. */
const openFolderButton = (page: Page) => testId(page, 'workspace.button');

// Specs share one app per worker, so a spec that leaves another folder open
// would hand the next one a workspace it never asked for.
test.afterEach(async ({ request, dataDir }) => {
	await call(request, `${WORKSPACE}.OpenFolder`, seeded(dataDir));
});

test('the workbench is there with no folder open', async ({ page, signIn, request, dataDir }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();
	await call(request, `${WORKSPACE}.CloseFolder`);

	// The chrome a signed-in user has, whether or not a folder is in it.
	await expect(screen(page)).toHaveAttribute('data-test-value', 'no_folder');
	await expect(openFolderButton(page)).toHaveText('Open folder');
	await expect(testId(page, 'tree.panel')).toBeVisible();
	await expect(page.getByText('Sam Okafor')).toBeVisible();

	// And the button in the corner opens one.
	await answerPicker(page, seeded(dataDir));
	await openFolderButton(page).click();
	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();
});

test('a picker that answers nothing leaves the button usable', async ({
	page,
	signIn,
	request
}) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();
	await call(request, `${WORKSPACE}.CloseFolder`);
	await expect(screen(page)).toHaveAttribute('data-test-value', 'no_folder');

	// A dialog the user closed without choosing. Never answering is the worst
	// case of that, and a spinner tied to it is one nothing can clear.
	const picks = { count: 0 };
	await intercept(page, PICK_FOLDER, async () => {
		picks.count++;
	});

	await openFolderButton(page).click();
	await expect.poll(() => picks.count).toBe(1);

	await openFolderButton(page).click();
	await expect.poll(() => picks.count).toBe(2);
});

test('the open-folder shortcut opens the picker', async ({ page, signIn, request }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();
	await call(request, `${WORKSPACE}.CloseFolder`);
	await expect(screen(page)).toHaveAttribute('data-test-value', 'no_folder');

	const picks = { count: 0 };
	await intercept(page, PICK_FOLDER, async () => {
		picks.count++;
	});

	await page.keyboard.press('ControlOrMeta+o');
	await expect.poll(() => picks.count).toBe(1);
});

test('signing in reopens the folder that was open', async ({ page, signIn }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();
	await expect(screen(page)).toHaveCount(0);
});

test('closing the folder offers it back', async ({ page, signIn, request, dataDir }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();
	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();

	await call(request, `${WORKSPACE}.CloseFolder`);

	await expect(screen(page)).toHaveAttribute('data-test-value', 'no_folder');
	await expect(page.getByText(seeded(dataDir))).toBeVisible();

	await page.getByRole('button', { name: 'Reopen analytics' }).click();
	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();
});

test('a folder with no config becomes a workspace', async ({ page, signIn, request, dataDir }) => {
	const folder = scratch(dataDir, 'fresh-');
	await readyToOpen(page, request, signIn, folder);

	await expect(screen(page)).toHaveAttribute('data-test-value', 'needs_setup');
	await expect(page.getByText(folder)).toBeVisible();

	// The folder's own name is the offer; typing over it is the exception.
	const name = page.getByPlaceholder('Workspace name');
	await expect(name).toHaveValue(basename(folder));
	await name.fill('reports');
	await page.getByRole('button', { name: 'Create workspace' }).click();

	// An empty folder is seeded on the way in, so the sample is what proves it opened.
	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();
	expect(JSON.parse(readFileSync(join(folder, 'select.config.json'), 'utf8')).server).toBe(
		currentServer(dataDir)
	);
});

test('a folder from another server is not opened here', async ({
	page,
	signIn,
	request,
	dataDir
}) => {
	await readyToOpen(page, request, signIn, join(dataDir, 'other-server'));

	await expect(screen(page)).toHaveAttribute('data-test-value', 'wrong_server');
	// Named twice on the screen: once as the folder's server, once in the fix.
	await expect(page.getByText('api.other.example.com').first()).toBeVisible();
	await expect(page.getByRole('button', { name: 'Sign out' })).toBeVisible();
});

test('a workspace the user is not in keeps its config', async ({
	page,
	signIn,
	request,
	dataDir
}) => {
	const folder = join(dataDir, 'revoked');
	await readyToOpen(page, request, signIn, folder);

	await expect(screen(page)).toHaveAttribute('data-test-value', 'no_access');

	// Replacing the config is a click away, never the button in front of the user.
	await expect(page.getByRole('button', { name: 'Create workspace' })).toHaveCount(0);
	expect(readFileSync(join(folder, 'select.config.json'), 'utf8')).toContain(
		'e2e-revoked-workspace'
	);
});

test('a workspace the server has never heard of is not opened', async ({ request, dataDir }) => {
	const folder = scratch(dataDir, 'gone-');
	writeFileSync(
		join(folder, 'select.config.json'),
		JSON.stringify({ version: 1, server: currentServer(dataDir), workspaceId: 'deleted-workspace' })
	);

	// Nothing local knows this workspace, so the app asks the server before
	// answering. It has to come back with a screen rather than an error.
	const state = await call<{ status: string }>(request, `${WORKSPACE}.OpenFolder`, folder);

	expect(state.status).toBe('no_access');
});
