import { expect, open, test, type Page } from '../../../tests/e2e/wails';
import { tabs } from '../../../tests/e2e/selectors';

/**
 * What a keystroke runs, on a keyboard that is not the one the bindings were
 * written on.
 *
 * A layout moves the punctuation: the key that prints "`" on a US keyboard
 * prints "@" on a French Mac, and "`" is somewhere else entirely. Both of those
 * keys are pressed here, as the browser reports them -- the character in `key`,
 * the physical key in `code` -- and both have to reach the binding written for
 * "ctrl+`".
 *
 * Driven with events rather than page.keyboard: the point is a pair the running
 * keyboard cannot produce, and a spec that could only press what this machine's
 * layout prints would be testing the layout it happens to run on.
 */

/** Presses a key as a given layout reports it. */
async function press(page: Page, code: string, key: string) {
	await page.evaluate(
		([code, key]) => {
			window.dispatchEvent(
				new KeyboardEvent('keydown', { code, key, ctrlKey: true, bubbles: true })
			);
		},
		[code, key]
	);
}

test('a binding is answered by the key that prints it, and by the key that holds it', async ({
	page,
	signIn
}) => {
	await open(page, signIn);
	await expect(tabs(page)).toHaveCount(0);

	// A layout that prints "`" on the key US keyboards print "7" on. The binding
	// names the character, so that is the key that runs it.
	await press(page, 'Digit7', '`');
	await expect(tabs(page)).toHaveCount(1);

	// The French Mac layout, where the key in the "`" position prints "@". It
	// prints nothing a binding could name, so it answers for where it sits.
	await press(page, 'Backquote', '@');
	await expect(tabs(page)).toHaveCount(2);

	// And a key that neither prints nor sits where the binding says runs nothing.
	await press(page, 'Digit7', '7');
	await expect(tabs(page)).toHaveCount(2);
});
