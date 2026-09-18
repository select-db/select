import { expect, holdSession, test, type Page } from '../../../../../../tests/e2e/wails';
import { testId } from '../../../../../../tests/e2e/selectors';

/**
 * Deleting the open workspace from its settings page.
 *
 * The one thing it must not do is end the session. DeleteWorkspace already
 * closes the folder, which is what puts the person on the no-folder screen with
 * their sign-in intact; the panel used to call Logout on top of that and take
 * the session with the workspace.
 */

const folderScreen = (page: Page) => testId(page, 'folder.screen');
const loginButton = (page: Page) => page.getByRole('button', { name: 'Log in with GitHub' });

test('deleting the open workspace leaves the person signed in', async ({ page, signIn }) => {
	await holdSession(page);
	await page.goto('/');
	await signIn();

	await page.getByRole('button', { name: 'Open Settings' }).click();
	await page.getByRole('button', { name: 'Workspace', exact: true }).click();
	await page.getByRole('button', { name: 'Delete workspace' }).click();
	await page.getByRole('button', { name: 'Delete', exact: true }).click();

	// The workspace is gone, so the folder closes and its screen says so.
	await expect(folderScreen(page)).toHaveAttribute('data-test-value', 'no_folder');

	// Still signed in: the login screen is what a logout would have put here.
	await expect(loginButton(page)).toBeHidden();
});
