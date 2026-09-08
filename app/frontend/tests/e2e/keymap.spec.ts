import { expect, open, test } from './wails';
import { activeTab, tab, tabs, testId, treeNode } from './selectors';

/**
 * The keymap: that the keys this platform sends reach the commands the keymap
 * binds them to.
 *
 * Everything a shortcut does is somebody else's spec — what is tested here is
 * the path from a keystroke to a command, and above all the bindings that are
 * written differently per platform. Those are the ones that rot: a chord that
 * is conventional on macOS is often taken on Linux, and a keymap that cannot
 * say so ships a key that quietly does nothing.
 *
 * The suite runs on Linux, so these are the Linux chords. `secondary` is Ctrl
 * here; on macOS the same commands answer to Cmd, and the two rows that differ
 * say so in their `when`.
 */

test('runs the commands the platform’s own chords are bound to', async ({ page, signIn }) => {
	await open(page, signIn);

	// Ctrl+N here, Cmd+N on macOS: one binding, written `secondary+n`.
	await page.keyboard.press('Control+n');
	await expect(activeTab(page)).toHaveAttribute('data-test-value', '[temp].sql');

	// Ctrl+W closes it, the same way.
	await page.keyboard.press('Control+w');
	await expect(tabs(page)).toHaveCount(0);

	// Ctrl+` opens a terminal. Written `ctrl+\``, which now means the Control
	// key on every platform rather than only on macOS.
	await page.keyboard.press('Control+`');
	await expect(tab(page, 'Terminal')).toBeVisible();
	await page.keyboard.press('Control+w');
	await expect(tabs(page)).toHaveCount(0);

	// Ctrl+J closes the left panel to nothing, and opens it again.
	await page.keyboard.press('Control+j');
	await expect(testId(page, 'tree.panel')).toBeHidden();
	await page.keyboard.press('Control+j');
	await expect(testId(page, 'tree.panel')).toBeVisible();
});

test('walks back and forward through tabs from the keyboard', async ({ page, signIn }) => {
	await open(page, signIn);

	await treeNode(page, 'weekly_revenue.sql').click();
	await expect(tab(page, 'weekly_revenue.sql')).toBeVisible();
	await treeNode(page, 'cohorts.sql').click();
	await expect(tab(page, 'cohorts.sql')).toBeVisible();

	// Ctrl+Alt+- and Ctrl+Shift+-, which is where every other editor puts them
	// on Linux and Windows. On macOS the keymap binds Ctrl+- and Ctrl+Shift+-
	// instead, since Ctrl+- is zoom out here.
	await page.keyboard.press('Control+Alt+Minus');
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'weekly_revenue.sql');

	await page.keyboard.press('Control+Shift+Minus');
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');

	// And the macOS chord does nothing here: it is bound `when os == 'macos'`,
	// and on this platform Ctrl+- is zoom out, which leaves the tabs alone.
	await page.keyboard.press('Control+Minus');
	await expect(activeTab(page)).toHaveAttribute('data-test-value', 'cohorts.sql');
});
