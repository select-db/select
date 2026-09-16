import { expect, holdSession as keepSession, test, type Locator, type Page } from './wails';
import { testId, editor, labelledInput } from './selectors';

/**
 * The shared half of the screenshot harness. The other half is one
 * `<shot-id>.shot.ts` per picture, living beside the content that shows it: see
 * `web/website/hero.shot.ts`.
 *
 * Everything here drives the real application, the same build every other spec
 * drives. Nothing is mocked except the AI provider, which `aiProvider.ts`
 * answers locally rather than faking in the UI.
 */

/**
 * The width the left bar opens at in every shot.
 *
 * The workspace button carries the workspace logo beside its name, which at the
 * stored default leaves no room for the name itself: every figure showed an
 * avatar and a truncation. A reader picks their own width; a screenshot cannot,
 * so it is set here rather than left to whatever the seed happened to store.
 */
const LEFTBAR_WIDTH = 290;

/**
 * Holds the session, as every spec does, and opens the left bar wide enough to
 * read the workspace name.
 *
 * Wrapped here rather than in `wails.ts`: the ordinary suite asserts against the
 * width a user gets, and only the pictures need a chosen one. An init script
 * because the bar reads the width once, as it mounts.
 *
 * `leftbar` is for a framing too narrow to give the bar this much without
 * squeezing what the picture is actually of.
 */
export async function holdSession(page: Page, leftbar = LEFTBAR_WIDTH) {
	await page.addInitScript((width) => localStorage.setItem('leftbarWidth', String(width)), leftbar);
	await keepSession(page);
}

/** Skipped unless SHOTS=1, so an ordinary test run never writes to the repo. */
export const shotsEnabled = () => Boolean(process.env.SHOTS);

export type Framing = {
	name: string;
	width: number;
	height: number;
	/** Render density. 1.5 is ~2x at the size the hero is actually displayed. */
	density?: number;
	/**
	 * Page zoom, as `Cmd/Ctrl +` applies it: the app lays out in fewer CSS
	 * pixels and draws each one larger. Raise it for a figure read in a narrow
	 * column, where the app at its own scale is too small to read.
	 *
	 * Steps of 1.2 match the app's own (see wails/zoom.ts), so a figure sits at
	 * a scale a reader can actually reach.
	 */
	appZoom?: number;
};

/**
 * The viewport and density a framing is captured at.
 *
 * Zoom is applied by laying the app out in fewer CSS pixels and rendering more
 * device pixels for each, which is what page zoom does: the app sees a smaller
 * window and draws everything larger, and the image comes out the same size.
 *
 * Not the CSS `zoom` property, which looks the same in a still and is not the
 * same thing: monaco positions its popups in unzoomed coordinates, so the
 * completion list drifts off the caret, and the results grid leaves gaps
 * between columns. A real window does neither.
 */
export function viewportFor(framing: Framing) {
	const zoom = framing.appZoom ?? 1;
	return {
		viewport: {
			width: Math.round(framing.width / zoom),
			height: Math.round(framing.height / zoom)
		},
		deviceScaleFactor: (framing.density ?? 1.5) * zoom
	};
}

export const THEMES = ['light', 'dark'] as const;
export type Theme = (typeof THEMES)[number];

/**
 * Where a shot writes: a `shots/` directory beside the file that declares it,
 * which is also beside the content that shows it. Derived from the spec's own
 * path, so moving a page and its shot moves the images with them and nothing
 * holds a stale relative path.
 */
export function shotsDirFor(specFile: string): string {
	return `${specFile.slice(0, specFile.lastIndexOf('/'))}/shots`;
}

/**
 * Fills a labelled input and asserts the value stuck.
 *
 * These fields re-render as a form revalidates, so a fill that lands on a
 * control about to be replaced leaves the placeholder showing -- which reads in
 * a screenshot as a required field nobody filled in.
 */
export async function fillLabelled(page: Page, label: string, value: string) {
	const input = labelledInput(page, label);
	await expect(input).toBeVisible();
	await input.fill(value);
	await expect(input).toHaveValue(value);
}

/**
 * Hides children of an expanded file-tree node through the app's own "n of m"
 * visibility badge. SQLite reports a second schema and five empty object
 * categories that would otherwise be most of the tree.
 */
export async function hideFileTreeChildren(page: Page, badgeIndex: number, names: string[]) {
	const badge = testId(page, 'tree.visibility-badge').nth(badgeIndex);
	for (const name of names) {
		const before = await badge.textContent();
		await badge.click();
		await page.getByText(name, { exact: true }).last().click();
		await page.keyboard.press('Escape');
		// The badge counts what is visible, so its text changing is the app
		// confirming the child is hidden. Nothing a fixed wait can assert.
		await expect(badge).not.toHaveText(before ?? '');
	}
}

/**
 * Applies the framing's app zoom, checks the two things that must never reach a
 * published image, and writes the file.
 *
 * No sleeps: every wait here is on something observable.
 */
export async function shot(
	page: Page,
	dir: string,
	name: string,
	framing: Framing,
	// The region to photograph, when the whole window is the wrong picture. The
	// hero spans the page's full width; the feature rows show their figure at
	// 550px, where a scaled-down window is unreadable and one pane is not.
	clip?: Locator
) {
	// Guard, not a workaround: .layout is `overflow: clip` now, so it cannot
	// scroll. It used to be `hidden`, which still makes a scroll container, and a
	// scrollIntoView from the tree left the whole app sitting 11px off-centre in
	// every shot. If that regresses this logs it rather than quietly baking it
	// into the images.
	const scrolled = await page.evaluate(() => {
		let moved = 0;
		for (const el of document.querySelectorAll<HTMLElement>('.layout, .wrapper')) {
			moved = Math.max(moved, el.scrollLeft);
			el.scrollLeft = 0;
		}
		return moved;
	});
	if (scrolled > 0) console.log(`layout was scrolled ${scrolled}px, reset before shot`);

	// A login screen is the one thing that must never be published.
	await expect(page.getByText('Log in with Github')).toBeHidden();

	// Monaco paints its first frame before its tokenizer runs, and for ~90ms
	// every token wears mtk1, the theme's default colour: SELECT and FROM as
	// plain text beside schema decorations that are already coloured. Nothing
	// else says the tokenizer has finished, so wait for a non-default token.
	if (await editor.tokens(page).count()) {
		await expect
			.poll(() =>
				editor
					.tokens(page)
					.evaluateAll((els) => els.some((el) => /\bmtk(?!1\b)\d+\b/.test(el.className)))
			)
			.toBe(true);
	}

	// `animations: 'disabled'` waits out CSS animations and transitions, or
	// fast-forwards them: the settle a trailing sleep used to approximate, done
	// by the tool that can observe it.
	const options = { path: `${dir}/${name}.png`, animations: 'disabled' as const };
	if (clip) await clip.screenshot(options);
	else await page.screenshot(options);
}

export { expect, test, type Locator, type Page };
