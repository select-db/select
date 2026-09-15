import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../app/frontend/tests/e2e/shots';
import { testId, editor } from '../../app/frontend/tests/e2e/selectors';

/**
 * The picture beside "It knows your schema before you do": the workspace's own
 * lint rules objecting to a draft query.
 *
 * It lives beside index.html, which shows it, and writes into shots/ beside
 * them both. Regenerate with `wails3 task shots` from app/.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/**
 * One framing, unlike the hero. This figure is shown at 550px on a desktop and
 * 770px when the row stacks, so it is captured at the larger of the two and
 * displayed at or below it. The window is wider than the picture: only the
 * editor pane is photographed, because a whole window shrunk to 550px is a
 * grey smear and one pane is legible.
 */
const FRAMING: Framing = { name: 'lint', width: 940, height: 470, density: 2 };

/** The rule the shipped .lint carries, and the draft that trips it. */
const RULE = 'Avoid SELECT *, list columns explicitly';

for (const theme of THEMES) {
	test.describe(`lint ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('a rule from the workspace, flagging a draft', async ({ page, signIn }, info) => {
			await holdSession(page);
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

			await testId(page, 'tree.node', 'cohorts.sql').click();
			await expect(editor.surface(page)).toBeVisible();

			// The marker comes from the Python analyzer the app shells out to,
			// through the workspace's .lint. Assert the underline itself: without
			// the analyzer the editor still opens the file, and a screenshot of an
			// unmarked draft would say the opposite of what the page claims.
			await expect(editor.warning(page)).toBeVisible({ timeout: 20_000 });

			// The message, not just the squiggle. At 550px an underline is a few
			// pixels; the sentence is the point.
			//
			// Moved to, not hovered: monaco draws the underline as a decoration the
			// mouse passes straight through, so playwright waits forever for an
			// element that will never receive the event. The tooltip answers to the
			// pointer being over those coordinates, which is what this does.
			const underline = await editor.warning(page).boundingBox();
			if (!underline) throw new Error('the lint underline has no box to hover');
			await page.mouse.move(underline.x + underline.width / 2, underline.y + underline.height / 2);
			await expect(page.getByText(RULE)).toBeVisible({ timeout: 10_000 });

			await shot(page, shotsDirFor(info.file), `lint.${theme}`, FRAMING, editor.surface(page));
		});
	});
}
