import {
	expect,
	holdSession,
	intercept,
	test,
	type Page,
	type Route
} from '../../../../tests/e2e/wails';
import { testId, treeRow } from '../../../../tests/e2e/selectors';

/**
 * Signing in from the login screen, which is a device code shown in a modal and
 * a poll waiting on the provider.
 *
 * The poll is the part that outlives the modal: a person closes it to reach
 * their browser, or dismisses it by clicking outside, and the sign-in they are
 * in the middle of has to still be running when they come back.
 */

/** Bound method ids, from src/lib/bindings/selectDb/internal/auth/githubauth.ts. */
const GET_DEVICE_CODE = 2636258938;
const GET_ACCESS_TOKEN = 2002237112;
const CANCEL_POLLING = 3744132403;

const DEVICE_CODE = {
	device_code: 'e2e-device-code',
	user_code: 'WDJB-MJHT',
	verification_uri: 'https://github.com/login/device',
	expires_in: 900,
	interval: 5
};

/** The activation code, which is nine separate boxes rather than one string. */
const code = (page: Page) => testId(page, 'login.code');

/** The boxes read as one character each, so the code reads back spaced out. */
const shown = (userCode: string) => userCode.split('').join(' ');

const loginButton = (page: Page) => page.getByRole('button', { name: 'Log in with GitHub' });
const openProvider = (page: Page) => page.getByRole('button', { name: 'Go to GitHub' });

type Provider = {
	codeCalls: number;
	pollCalls: number;
	cancelCalls: number;
	/** Answers the outstanding poll, as the provider does once the user says yes. */
	authorize: () => Promise<void>;
};

/**
 * Stands in for GitHub: hands out one device code and then leaves the poll
 * outstanding until the test authorizes, which is the state a person leaves the
 * app in when they go to their browser. Counts every call, since what the app
 * must not do here is call cancel.
 */
async function githubWaiting(page: Page): Promise<Provider> {
	let poll: Route | undefined;
	let answered: (() => void) | undefined;

	const calls: Provider = {
		codeCalls: 0,
		pollCalls: 0,
		cancelCalls: 0,
		authorize: async () => {
			await poll?.fulfill({ status: 200, body: '' });
			answered?.();
		}
	};

	await intercept(page, GET_DEVICE_CODE, async (route: Route) => {
		calls.codeCalls += 1;
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(DEVICE_CODE)
		});
	});
	await intercept(page, GET_ACCESS_TOKEN, async (route: Route) => {
		calls.pollCalls += 1;
		poll = route;
		await new Promise<void>((resolve) => (answered = resolve));
	});
	await intercept(page, CANCEL_POLLING, async (route: Route) => {
		calls.cancelCalls += 1;
		await route.fulfill({ status: 200, body: '' });
	});

	return calls;
}

/** Opens the login modal and waits for the code the provider handed out. */
async function startSigningIn(page: Page) {
	await loginButton(page).click();
	await expect(code(page)).toHaveText(shown(DEVICE_CODE.user_code));
}

test('closing the modal leaves the sign-in running', async ({ page, signIn }) => {
	await holdSession(page);
	const github = await githubWaiting(page);
	await page.goto('/');

	await startSigningIn(page);
	expect(github.pollCalls).toBe(1);

	// Dismissed the way a person reaching for their browser dismisses it.
	await page.mouse.click(5, 5);
	await expect(openProvider(page)).toBeHidden();

	expect(github.cancelCalls).toBe(0);

	// Reopening shows the same code rather than asking for a second one, which
	// would invalidate the one already being typed at the provider.
	await loginButton(page).click();
	await expect(code(page)).toHaveText(shown(DEVICE_CODE.user_code));
	expect(github.codeCalls).toBe(1);
	expect(github.pollCalls).toBe(1);

	// The poll was never cancelled, so authorizing at the provider still signs in.
	await github.authorize();
	await signIn();
	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();
});

test('cancelling the modal ends the sign-in', async ({ page }) => {
	await holdSession(page);
	const github = await githubWaiting(page);
	await page.goto('/');

	await startSigningIn(page);

	await page.getByRole('button', { name: 'Cancel' }).click();
	await expect(openProvider(page)).toBeHidden();
	expect(github.cancelCalls).toBe(1);

	// Nothing is left running, so the next attempt asks for a fresh code.
	await startSigningIn(page);
	expect(github.codeCalls).toBe(2);
});
