import { expect, test as base, type APIRequestContext, type Page } from '@playwright/test';

/**
 * Talking to the Go side the way the app does — over `/wails/runtime`, the
 * endpoint the app's own bindings call — but from the test process rather than
 * from inside the page.
 *
 * That distinction matters. The runtime is also served to the page at
 * `/wails/runtime.js`, and importing it from a test looks tempting; it also
 * registers a second client id and takes over the event dispatcher, after which
 * the app's own calls start coming back 422. Speaking HTTP leaves the page
 * exactly as the app left it, at the cost of naming two protocol constants.
 */
const CALL_OBJECT = 0;
const EVENTS_OBJECT = 3;
const FIRST_METHOD = 0;

/** Calls a bound Go method by name, e.g. `pkg.Service.Method`. */
export async function call<T = unknown>(
	request: APIRequestContext,
	method: string,
	...args: unknown[]
): Promise<T> {
	const response = await request.post('/wails/runtime', {
		headers: { 'x-wails-client-id': 'e2e' },
		data: {
			object: CALL_OBJECT,
			method: FIRST_METHOD,
			args: { 'call-id': method, methodName: method, args }
		}
	});

	const body = await response.text();
	if (!response.ok()) throw new Error(`${method}: ${response.status()} ${body}`);

	// Scalar results come back as bare text, everything else as JSON.
	return (response.headers()['content-type']?.includes('json') ? JSON.parse(body) : body) as T;
}

export const test = base.extend<{
	emit: (name: string, data?: unknown) => Promise<void>;
	signIn: () => Promise<void>;
	consoleErrors: string[];
}>({
	/**
	 * Emits an event, standing in for a backend that emitted it itself: the Go
	 * side broadcasts it to every listener, the app included.
	 *
	 * Events are never queued or replayed, and in server mode the socket that
	 * carries them and the app bundle that listens on it come up independently —
	 * so an event can reach a page that is not listening yet, and is then simply
	 * dropped. There is no earlier moment to wait for instead: the only reliable
	 * signal is the app acting on it. Callers therefore emit inside a poll and
	 * stop once they see the effect, which is why every event used here has to
	 * be safe to send more than once.
	 */
	emit: async ({ request }, use) => {
		await use(async (name, data) => {
			await request.post('/wails/runtime', {
				headers: { 'x-wails-client-id': 'e2e' },
				// An absent `data` would drop out of the JSON, which the Go side rejects.
				data: { object: EVENTS_OBJECT, method: FIRST_METHOD, args: { name, data: data ?? null } }
			});
		});
	},

	/**
	 * Puts the app into its signed-in state.
	 *
	 * `login` is the event the frontend's session wall listens for; the Go side
	 * emits it once it finds a stored token and a current user. Tokens live in
	 * the OS keyring, which a headless runner has none of, so the suite emits the
	 * event and lets the app read the seeded database for the rest. The login
	 * screen giving way is the app confirming it was listening.
	 */
	signIn: async ({ page, emit }, use) => {
		await use(async () => {
			const loginScreen = page.getByText('Log in with Github');
			await loginScreen.waitFor();

			await expect
				.poll(async () => {
					await emit('login');
					return loginScreen.isVisible();
				})
				.toBe(false);
		});
	},

	consoleErrors: [
		async ({ page }, use) => {
			const errors: string[] = [];
			page.on('console', (message) => message.type() === 'error' && errors.push(message.text()));
			page.on('pageerror', (error) => errors.push(`uncaught: ${error.message}`));
			await use(errors);
		},
		{ auto: true }
	]
});

export { expect };
export type { Page };
