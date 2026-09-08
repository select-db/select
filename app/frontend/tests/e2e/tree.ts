import { expect, type Page } from './wails';
import { renameBox, testId, treeNode } from './selectors';

/**
 * Driving the workspace tree: the handful of gestures every spec that touches
 * it needs, so they mean the same thing in all of them.
 *
 * `selectors.ts` says how to address a row; this says what to do to one. The
 * split matters because these carry waits and assumptions -- a menu that has to
 * be matched exactly, a rename box that opens with part of the name selected --
 * and a spec that reimplements them gets a subtly different gesture with the
 * same name.
 */

/** The tree's own context menu, from the empty space below the last row. */
export async function openTreeMenu(page: Page) {
	const panel = testId(page, 'tree.panel');
	const box = await panel.boundingBox();
	if (!box) throw new Error('file tree is not on screen');

	await panel.click({ button: 'right', position: { x: 20, y: box.height - 20 } });
}

/** The context menu of one row. */
export async function openMenuOn(page: Page, name: string) {
	await treeNode(page, name).click({ button: 'right' });
}

/**
 * Picks an entry from whichever menu is open.
 *
 * Exactly, because the tree's menus carry both "Delete" and "Delete selected":
 * a loose match on the first would sometimes hit the second and take rows with
 * it that nobody named.
 */
export async function chooseMenuItem(page: Page, name: string) {
	await page.getByRole('menuitem', { name, exact: true }).click();
}

/**
 * Types a whole new name into the rename box a new file, a new folder and
 * "Rename..." all open, and commits it.
 *
 * The box opens with part of the name selected -- up to the extension, so
 * typing keeps it -- which is right for a person and ambiguous for a test,
 * hence the select-all first: what is typed here is the whole new name.
 *
 * The box closes whether the name was taken or not, so this returns once the
 * app has answered, not once it has agreed.
 */
export async function renameTo(page: Page, name: string) {
	const box = renameBox(page);
	await expect(box).toBeFocused();

	await box.press('ControlOrMeta+a');
	await box.fill(name);
	await box.press('Enter');

	await expect(box).toBeHidden();
}

/** Leaves the rename box without renaming, which is how a default name is kept. */
export async function keepName(page: Page) {
	const box = renameBox(page);
	await expect(box).toBeFocused();
	await box.press('Escape');
	await expect(box).toBeHidden();
}
