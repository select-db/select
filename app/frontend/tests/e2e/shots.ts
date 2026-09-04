import { expect, holdSession, test, type Locator, type Page } from './wails';
import { testId, editor, labelledInput } from './selectors';

/**
 * The shared half of the screenshot harness. The other half is one
 * `<shot-id>.shot.ts` per picture, living beside the content that shows it —
 * see `web/site/hero.shot.ts`.
 *
 * Everything here drives the real application: the same server-mode build, Go
 * services, bindings and events every other spec drives. Nothing is mocked
 * except the AI provider, which is answered locally rather than faked in the
 * UI.
 */

// `tests/**` is type-checked by svelte-check, which has no Node types — the app
// has no reason to depend on them. Declaring the one global the harness needs
// keeps it that way.
declare const process: { env: Record<string, string | undefined> };

// Re-exported because every shot holds the session, and a shot spec should not
// have to import from a second file to do the one thing all of them do.
export { holdSession };

/** Skipped unless SHOTS=1, so an ordinary test run never writes to the repo. */
export const shotsEnabled = () => Boolean(process.env.SHOTS);

export type Framing = {
	name: string;
	width: number;
	height: number;
	/** Render density. 1.5 is ~2x at the size the hero is actually displayed. */
	density?: number;
	/** App UI scale. Raise it for a doc figure read in a narrow column: fewer
	 *  pixels of chrome, larger text, without shrinking the window. */
	appZoom?: number;
};

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
 * Answers the AI provider from here. Nothing reaches a real API: no key, no
 * network, no cost, and the same reply on every run, which a live model could
 * never give. The app still requires a key to be present before it will try,
 * and reads it from the workspace .env, where the seed leaves a placeholder.
 *
 * Turns are served in order, so a shot can hand back a tool call first and let
 * the app run it for real before answering.
 */
export async function stubChatProvider(page: Page, turns: string[]) {
	let turn = 0;
	await page.route('https://api.anthropic.com/**', async (route) => {
		const body = turns[Math.min(turn, turns.length - 1)];
		turn += 1;
		await route.fulfill({
			status: 200,
			headers: { 'content-type': 'text/event-stream' },
			body
		});
	});
}

const sse = (type: string, data: object) =>
	`event: ${type}\ndata: ${JSON.stringify({ type, ...data })}\n\n`;

/** An assistant turn that calls one tool, for the app to run for real. */
export const toolCallTurn = (name: string, input: object, id = 'toolu_e2e_1') =>
	sse('content_block_start', { index: 0, content_block: { type: 'tool_use', id, name } }) +
	sse('content_block_delta', {
		index: 0,
		delta: { type: 'input_json_delta', partial_json: JSON.stringify(input) }
	}) +
	sse('message_delta', { delta: { stop_reason: 'tool_use' } });

/** An assistant turn that just answers. */
export const textTurn = (text: string) =>
	sse('content_block_delta', { index: 0, delta: { type: 'text_delta', text } }) +
	sse('message_delta', { delta: { stop_reason: 'end_turn' } });

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
	if (framing.appZoom) {
		await page.evaluate((z) => {
			document.documentElement.style.zoom = String(z);
		}, framing.appZoom);
		await expect
			.poll(() => page.evaluate(() => document.documentElement.style.zoom))
			.toBe(String(framing.appZoom));
	}

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

	// Monaco paints its first frame before its tokenizer has run, and for about
	// 90ms every token wears mtk1, the theme's default colour: a picture taken in
	// that window shows SELECT and FROM as plain text while the schema
	// decorations around them are already coloured, which reads as an editor that
	// does not know it is looking at SQL. Nothing else on screen says it has
	// finished, so wait for a token that is not the default one. Skipped when
	// there is no editor, or when the editor is empty.
	if (await editor.tokens(page).count()) {
		await expect
			.poll(() =>
				editor
					.tokens(page)
					.evaluateAll((els) => els.some((el) => /\bmtk(?!1\b)\d+\b/.test(el.className)))
			)
			.toBe(true);
	}

	// `animations: 'disabled'` waits for CSS animations and transitions to
	// finish, or fast-forwards them — the settle that a trailing sleep used to
	// approximate, done by the tool that can actually observe it.
	const shot = { path: `${dir}/${name}.png`, animations: 'disabled' as const };
	if (clip) await clip.screenshot(shot);
	else await page.screenshot(shot);
}

export { expect, test, type Locator, type Page };
