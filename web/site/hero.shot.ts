import {
	shot,
	shotsEnabled,
	hideFileTreeChildren,
	holdSession,
	shotsDirFor,
	stubChatProvider,
	textTurn,
	toolCallTurn,
	THEMES,
	expect,
	test,
	type Framing
} from '../../app/frontend/tests/e2e/shots';
import { testId, editor } from '../../app/frontend/tests/e2e/selectors';

/**
 * The picture at the top of the landing page, captured from the running
 * application. It lives beside `index.html`, which shows it, and writes into
 * `shots/` beside them both — so a change to the page and a change to its
 * picture land in the same directory and read as one diff.
 *
 * Regenerate with `wails3 task shots` from app/, then commit what changed. The
 * task rebuilds the binary the capture drives first, which `npm run shots`
 * does not: a stale one does not fail, it photographs the previous build.
 */
test.skip(!shotsEnabled(), 'set SHOTS=1 (wails3 task shots) to capture screenshots');

/**
 * Two framings rather than two resolutions. A desktop IDE shrunk to phone width
 * is unreadable, so the narrow capture is the app laid out narrow — a different
 * picture, which is why the page selects between them with `media` rather than
 * leaving it to `srcset`.
 *
 * 1.5x rather than 2x: the hero displays about 1080 CSS px wide, so 1440 x 1.5
 * is already ~2x at the size it is actually shown, and 2x of the capture width
 * would be oversampling nobody sees.
 */
/**
 * Where the WHERE clause is supposed to end. The capture types a predicate onto
 * this line to open the completion popup, so anything after this literal is
 * left over from a previous pass and has to go before this one starts.
 */
const CLAUSE_END = "'2026-01-05'";

/**
 * The clause boundary with whitespace collapsed. Trimming the line the capture
 * types on is not enough on its own: what a half-written file actually breaks
 * is the join between the WHERE clause and the GROUP BY after it, and the
 * formatter will happily glue a stray character to the next keyword. Checking
 * the boundary turns that into an immediate failure instead of a 25-second
 * timeout on a result row that was never going to arrive.
 *
 * Whitespace-insensitive because the formatter has two layouts for this query
 * and both are correct.
 */
const CLAUSE_BOUNDARY = "WHERE o.created_at >= '2026-01-05' GROUP BY week";

/**
 * fs_provider.Write, from the generated bindings. The editor saves on a 200ms
 * debounce, so the predicate this capture types is not on disk the moment the
 * picture is taken. Nothing here restores the file, so what the next pass finds
 * would otherwise depend on where that debounce landed relative to teardown.
 */
const FS_WRITE = 2205286156;

/**
 * `chat` is local to this capture, not part of a framing: at 860px the editor
 * and a chat panel cannot both be worth looking at, and the query is what the
 * picture is about. The narrow cut is the same app with the panel closed, which
 * is also how anyone would actually work at that width.
 */
const FRAMINGS: (Framing & { chat: boolean })[] = [
	{ name: 'wide', width: 1440, height: 900, chat: true },
	{ name: 'narrow', width: 860, height: 760, chat: false }
];

/**
 * Two turns, because one is not the product. The first answers with a tool
 * call, which the app runs for real against the seeded database — the card in
 * the picture is genuine introspection, not a drawing of one. The second, now
 * holding that result, answers.
 *
 * The answer claims nothing the data does not support: `orders.status` and the
 * partial trailing week are both real.
 */
const TURNS = [
	toolCallTurn('get_database_schemas', { databaseInstanceId: 'sample-warehouse' }),
	textTurn(
		'`orders` has a `status` column, and `refunded` is in it, so this query is ' +
			'gross revenue, not net. Two fixes worth making:\n\n' +
			"1. Add `WHERE o.status = 'paid'` to exclude refunds.\n" +
			'2. The trailing week is partial, so its row is not comparable to the rest.\n\n' +
			'`order_items` also carries `quantity` and `unit_cents`, if you want units ' +
			'shifted alongside revenue.'
	)
];

for (const framing of FRAMINGS) {
	for (const theme of THEMES) {
		test.describe(`hero ${framing.name} ${theme}`, () => {
			test.use({
				viewport: { width: framing.width, height: framing.height },
				deviceScaleFactor: framing.density ?? 1.5
			});

			// One pass per theme rather than one pass photographed twice: switching
			// theme re-renders the editor, which closes the completion popup and
			// takes focus with it. Setting it before anything else is driven avoids
			// having to restore state the repaint destroyed.
			test('a query, its results, and completion over both', async ({ page, signIn }, info) => {
				await holdSession(page);
				if (framing.chat) {
					await stubChatProvider(page, TURNS);
				}
				await page.goto('/');
				await signIn();

				await page.evaluate((t) => document.documentElement.setAttribute('data-theme', t), theme);
				// The theme store re-applies its variables from a MutationObserver on
				// the attribute, so the repaint lands a tick later. Wait for the paint
				// itself rather than for a guessed number of milliseconds.
				await expect
					.poll(() =>
						page.evaluate(() => {
							const [r, g, b] = (
								getComputedStyle(document.body).backgroundColor.match(/\d+/g) ?? [
									'255',
									'255',
									'255'
								]
							).map(Number);
							return (r + g + b) / 3 < 128 ? 'dark' : 'light';
						})
					)
					.toBe(theme);

				// Open the database down to the table the query reads, hiding the
				// categories SQLite reports but the demo never fills.
				// Each step waits for what the previous one produced: introspection is
				// a round trip to the database, and how long it takes is not this
				// file's business.
				// Tree rows carry their name, so each step names the row it wants
				// instead of matching loose text and taking the first hit.
				const node = (name: string) => testId(page, 'tree.node', name);
				// Expanding a node introspects the database, so these wait on I/O, not
				// on rendering. A longer ceiling costs nothing when it is fast and
				// stops a loaded machine looking like a broken capture.
				const appeared = (name: string) => expect(node(name)).toBeVisible({ timeout: 20_000 });

				await node('warehouse').click();
				await appeared('main');
				await hideFileTreeChildren(page, 0, ['sqlite_builtin']);

				await node('main').click();
				await appeared('Tables');
				await hideFileTreeChildren(page, 1, ['Views', 'Triggers', 'Types', 'Functions', 'Pragmas']);

				await node('Tables').click();
				await appeared('orders');
				await node('orders').click();
				await appeared('Columns');
				await node('Columns').click();
				await appeared('total_cents');

				await node('weekly_revenue.sql').click();
				await expect(editor.surface(page)).toBeVisible();

				// Every pass drives the same workspace on disk, and this one is about
				// to type a predicate onto the WHERE clause. Clear whatever the last
				// pass left there, rather than making that pass put it back: undoing
				// the edit afterwards, and then waiting for the debounced save to
				// prove the file was clean, cost more than the capture itself, and
				// bought nothing that this does not.
				//
				// How much to delete is measured, not remembered: everything after the
				// clause's closing quote. A pass that ended early, or a keystroke the
				// suggest widget swallowed, leaves a different number of characters
				// each time, and a fixed count would either miss one or eat into the
				// date. Retried as a whole because it re-measures, so a retry cannot
				// over-delete.
				const clause = editor.line(page, '2026-01-05').first();
				await expect(async () => {
					const text = await clause.innerText();
					const extra = text.length - (text.indexOf(CLAUSE_END) + CLAUSE_END.length);
					if (extra > 0) {
						// Armed before the keystrokes, because the save is debounced
						// 200ms behind the last one.
						const written = page.waitForRequest(
							(r) => (r.postData() ?? '').includes(`"methodID":${FS_WRITE}`),
							{ timeout: 10_000 }
						);
						await clause.click();
						await page.keyboard.press('End');
						for (let i = 0; i < extra; i++) {
							await page.keyboard.press('Backspace');
						}
						// Fixing the buffer is not enough: the app re-reads this file
						// from disk later in the pass, and an unsaved fix is silently
						// replaced by the predicate it just removed. The picture then
						// shows two of them, which is how this was found.
						await written;
					}
					expect(await clause.innerText()).toMatch(/'2026-01-05'$/);

					const query = (await editor.lines(page).innerText()).replace(/\s+/g, ' ');
					expect(query, 'the query in the workspace is not the fixture').toContain(CLAUSE_BOUNDARY);
				}).toPass({ timeout: 15_000 });

				await editor.lines(page).click({ position: { x: 4, y: 300 } });

				// Format, the way anyone would before running: cmd+s is bound to
				// editor.formatDocument, so the query in the picture is the
				// formatter's output rather than however the fixture was written.
				// Assert the formatter's shape, not a delta: the file is left formatted
				// for the next pass, so "the line count went up" only holds the first
				// time. Formatting puts FROM on a line of its own.
				// Formatting is an edit like any other, so it is not durable until the
				// 200ms debounce writes it. Wait for that write before moving on:
				// opening the chat makes the app re-read this file from disk, and an
				// unsaved format is replaced by the unformatted text it just rewrote.
				// That is how the wide screenshots came to show a query the capture
				// had demonstrably just formatted, while the narrow ones, which open
				// no chat, showed it formatted.
				//
				// Only the first pass does real work here: it leaves the file
				// formatted for the ones after it, where cmd+s changes nothing and no
				// save follows. Waiting for a write that is never coming would hang,
				// so ask first whether there is anything to wait for.
				const onOwnLine = editor.line(page).filter({ hasText: /^\s*FROM\s*$/ });
				const alreadyFormatted = (await onOwnLine.count()) === 1;
				const formatted = alreadyFormatted
					? null
					: page.waitForRequest((r) => (r.postData() ?? '').includes(`"methodID":${FS_WRITE}`), {
							timeout: 10_000
						});

				await page.keyboard.press('ControlOrMeta+s');
				await expect(onOwnLine).toHaveCount(1);
				await formatted;

				// Run first: the result results table is part of the picture.
				await page.keyboard.press('ControlOrMeta+Enter');
				await expect(page.getByText('revenue', { exact: true }).first()).toBeVisible({
					timeout: 15_000
				});
				// The header arrives with the results table; the rows stream in after it, and an
				// empty results table of placeholder dashes is not what the page is claiming.
				// Any week cell will do — pinning to the first row couples this to the
				// results table's virtualisation, which is what made it flake in the narrow
				// framing.
				await expect(page.getByText(/^2026-\d{2}-\d{2}$/).first()).toBeVisible({
					timeout: 25_000
				});

				if (framing.chat) {
					// A chat alongside the query, answering about the result on screen.
					// The tab-bar actions carry a tooltip, not an accessible name, so
					// address the container the markup names: chat first, terminal second.
					await testId(page, 'tabs.actions').getByRole('button', { name: 'Open Chat' }).click();
					const prompt = page.getByRole('textbox', { name: 'Type a message...' });
					await expect(prompt).toBeVisible();
					await prompt.click();
					// No delay: this input has no completion to keep up with, and a
					// keystroke rhythm nothing observes is a second per pass.
					await page.keyboard.type(
						'Revenue dipped the week of 2026-02-09. Anything off about this query?'
					);
					await page.keyboard.press('Enter');

					// The tool card is the app running get_database_schemas against the
					// seeded warehouse; the answer only arrives on the turn after it.
					await expect(page.getByText('Read database schemas').first()).toBeVisible({
						timeout: 20_000
					});
					await expect(page.getByText('gross revenue', { exact: false })).toBeVisible({
						timeout: 20_000
					});

					// Narrow the chat: at its default width it takes half the frame, and
					// the query is what the picture is about. The split's own resizer, not
					// the side bars': all three carry the class, and only this one sits
					// inside a split container.
					const resizer = testId(page, 'split.resizer').first();
					const box = await resizer.boundingBox();
					if (!box) throw new Error('no split resizer: did the chat open beside the editor?');
					await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
					await page.mouse.down();
					await page.mouse.move(Math.round(framing.width * 0.71), box.y + box.height / 2, {
						steps: 5
					});
					await page.mouse.up();
					// The drag lands when the resizer has actually moved, which is a
					// position to assert, not a duration to guess.
					await expect
						.poll(async () => Math.round((await resizer.boundingBox())?.x ?? 0))
						.toBeGreaterThan(Math.round(framing.width * 0.6));
				}

				// Armed before the edit, not after: the debounce fires 200ms after the
				// last keystroke, which is while the completion popup is still being
				// asserted below. A listener attached after that would be waiting for
				// a write that has already gone out.
				const saved = page.waitForRequest(
					(r) => (r.postData() ?? '').includes(`"methodID":${FS_WRITE}`),
					{ timeout: 10_000 }
				);

				// Leave the caret mid-completion, extending the WHERE clause. It has to
				// go somewhere the statement still parses: an incomplete select list
				// resolves no alias, and the popup falls back to a bare snippet.
				await editor.line(page, '2026-01-05').first().click();
				await page.keyboard.press('End');
				const typed = ' AND o.';
				await page.keyboard.type(typed, { delay: 60 });

				// Completion comes from the Python analyzer the app shells out to. If
				// it is missing the popup still opens, with one static snippet in it —
				// so assert on a real column rather than on the widget, or a run
				// without the analyzer quietly publishes a picture of the app not
				// doing the thing the page says it does.
				await expect(editor.completionItem(page, 'total_cents')).toBeVisible({ timeout: 15_000 });

				// Exactly one predicate, on the line the picture is about. Everything
				// else in this pass stayed true when a stale reload put a second one
				// there, and the doubled query was published before anyone noticed.
				// Monaco renders spaces as non-breaking ones, so a literal space in a
				// pattern never matches what innerText returns.
				// Still formatted at the shutter, not merely formatted at some point.
				// Opening the chat re-reads the file, and this is the assertion that
				// catches the formatting being thrown away between the two.
				await expect(onOwnLine).toHaveCount(1);

				const clauseText = (await editor.line(page, '2026-01-05').first().innerText()).replace(
					/\u00a0/g,
					' '
				);
				expect(clauseText.match(/ AND o\./g) ?? [], clauseText).toHaveLength(1);

				// The toolbar reports how long the query took. Over twelve rows that is
				// one to three milliseconds depending on what else the machine is
				// doing, and that one digit was the only thing that differed between
				// runs of this capture: it rewrote a 220KB PNG every time, for a
				// number nobody reads. Assert the app measured and rendered a real
				// duration, then pin the digits, so a regenerated screenshot changes
				// when the product does and not otherwise. The pinned value is one
				// this query actually produces.
				//
				// Immediately before the capture, not next to the query it times: the
				// component re-renders while the chat runs, and an earlier pin is
				// quietly replaced by the real measurement again.
				const duration = testId(page, 'result.duration');
				await expect(duration).toHaveText(/^\d\d:\d\d\.\d\d\d$/);
				await duration.evaluate((el) => {
					el.textContent = '00:00.002';
				});

				await shot(page, shotsDirFor(info.file), `hero.${framing.name}.${theme}`, framing);

				// Leave the workspace in a state the next pass can reason about. It
				// does not need to be clean, only settled: an edit still in flight
				// when the context closes is the one thing normalising cannot undo.
				await saved;
			});
		});
	}
}
