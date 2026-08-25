import { test as base, type APIRequestContext, type Page } from '@playwright/test';

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
	 * The app receives events over a WebSocket its runtime opens shortly after
	 * the page loads — server mode has no native bridge to deliver them — so an
	 * event emitted too early would reach nobody. Waiting for that socket is
	 * what makes this deterministic.
	 */
	emit: async ({ page, request }, use) => {
		const connected = page.waitForEvent('websocket', (socket) =>
			socket.url().includes('/wails/events')
		);

		await use(async (name, data) => {
			await connected;
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
	 * the OS keyring, which a headless runner has none of, so the suite emits
	 * the event and lets the app read the seeded database for the rest. Events
	 * are not replayed, so the wall has to be listening first — its login screen
	 * is the proof that it is.
	 */
	signIn: async ({ page, emit }, use) => {
		await use(async () => {
			await page.getByText('Log in with Github').waitFor();
			await emit('login');
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

export { expect } from '@playwright/test';
export type { Page };
