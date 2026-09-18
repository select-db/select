import { expect, holdSession, test } from './wails';

/**
 * How much JavaScript the app parses before it can paint.
 *
 * It was 5.5MB, and 580ms of blank window, because the shell reached every
 * view and monaco through four separate import paths. Each of those is behind
 * a dynamic import now, and nothing else would notice if one came back: the
 * app would still work, still pass every other spec here, and simply take
 * three times as long to open.
 *
 * The budget is a ceiling with room to grow in, not a target -- what it is
 * really watching for is a library arriving, since the four that were removed
 * are 400KB each at the smallest.
 */
const BUDGET_KB = 1000;

test('the app paints before it has parsed the whole product', async ({ page }) => {
	await holdSession(page);
	await page.goto('/');
	await page.getByText('Log in with Github').waitFor();

	const kb = await page.evaluate(() =>
		Math.round(
			(performance.getEntriesByType('resource') as PerformanceResourceTiming[])
				.filter((r) => r.name.endsWith('.js'))
				.reduce((total, r) => total + (r.decodedBodySize || 0), 0) / 1024
		)
	);

	expect(kb, `${kb}KB of JavaScript loaded before first paint`).toBeLessThan(BUDGET_KB);
});
