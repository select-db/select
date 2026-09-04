import {
	shot,
	shotsEnabled,
	holdSession,
	shotsDirFor,
	THEMES,
	expect,
	test,
	type Framing
} from '../../frontend/tests/e2e/shots';
import { testId } from '../../frontend/tests/e2e/selectors';

/**
 * The picture beside "Review a query like you review code": the workspace's
 * source control, with a branch, a commit the remote has not seen, and a change
 * being reviewed as a diff.
 *
 * Everything in it comes from a real repository. See initWorkspaceRepo in
 * internal/cmd/e2eseed, which makes the seeded workspace a git repository with
 * exactly this shape.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/**
 * A small window on purpose. This figure is shown at 550px, so the app has to
 * be captured near that size to stay legible; a 1440px window scaled to 550 is
 * unreadable. The panel and the diff both have to be in frame, which is what
 * sets the width.
 */
const FRAMING: Framing = { name: 'git', width: 940, height: 520, density: 2 };

/** What the seed left uncommitted, and the line it added. */
const EDITED = 'top_customers.sql';
/** The line the working copy adds, which the diff has to be showing. */
const ADDED_LINE = "o.status = 'paid'";

for (const theme of THEMES) {
	test.describe(`git ${theme}`, () => {
		test.use({
			viewport: { width: FRAMING.width, height: FRAMING.height },
			deviceScaleFactor: FRAMING.density ?? 1.5
		});

		test('a branch, a change, and the diff for it', async ({ page, signIn }, info) => {
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

			await page.getByRole('button', { name: 'Source control' }).click();
			await expect(testId(page, 'git.panel')).toBeVisible();

			// The branch the seed created, so this is the repository's own state
			// rather than a panel that happens to be open.
			await expect(page.getByText('feat/cohort-report')).toBeVisible({ timeout: 15_000 });

			// Open the change as a diff: reviewing it is what the row is about.
			await testId(page, 'tree.node', EDITED).click();
			await expect(page.getByText(ADDED_LINE).first()).toBeVisible({ timeout: 15_000 });

			await shot(page, shotsDirFor(info.file), `git.${theme}`, FRAMING);
		});
	});
}
