import {
	expect,
	test as base,
	type APIRequestContext,
	type Locator,
	type Page
} from '@playwright/test';

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

/**
 * Signed-in state survives only while a token exists. Tokens live in the OS
 * keyring, which a headless runner has none of, so the app's 500ms
 * CheckForLogout poll emits `logout` about half a second in and everything
 * after runs on a login screen. Answering that one call keeps the session up
 * without changing the app's behaviour.
 *
 * The number is wails' id for the bound method, from the generated
 * `src/lib/bindings/selectDb/internal/system/system.ts`. Wails derives it from
 * the method's qualified name, so it is identical across builds and changes
 * only if System.CheckForLogout is renamed or moved -- at which point every
 * shot fails on a login screen. Regenerate the bindings and copy the new id.
 */
const CHECK_FOR_LOGOUT = 2480583021;

export async function holdSession(page: Page) {
	await page.route('**/wails/runtime', async (route) => {
		if ((route.request().postData() ?? '').includes(`"methodID":${CHECK_FOR_LOGOUT}`)) {
			await route.fulfill({ status: 200, body: '' });
			return;
		}
		await route.continue();
	});
}

/**
 * The two services a spec asks about the workspace. Qualified Go names, which
 * is what the runtime dispatches on.
 */
const FS = 'selectDb/internal/fs_provider.FSProvider';
export const GRAPH = 'selectDb/internal/graph.Graph';

/** The id of the workspace the seed left, which every path below hangs off. */
export async function workspaceId(request: APIRequestContext): Promise<string> {
	const workspace = await call<{ id: string }>(request, `${GRAPH}.GetWorkspaceGraph`);
	return workspace.id;
}

/**
 * Runs a command in the workspace root, standing in for everything that changes
 * a workspace without going through the app: a terminal, a git checkout, an
 * editor somebody else has open. The app only finds out by watching.
 */
export async function inWorkspace(
	request: APIRequestContext,
	id: string,
	command: string,
	...args: string[]
) {
	const result = await call<{ exitCode: number; stderr: string }>(request, `${FS}.ExecuteCommand`, {
		workspaceId: id,
		command,
		args
	});
	if (result.exitCode !== 0) {
		throw new Error(`${command} ${args.join(' ')}: ${result.stderr}`);
	}
}

/**
 * The URI of a root-relative path in a workspace. The prefix is fixed for the
 * whole run, and readWorkspaceFile is called inside a poll, so it is fetched
 * once rather than on every call.
 */
let uriPrefix: Promise<string> | null = null;

async function workspaceURI(request: APIRequestContext, id: string, path: string) {
	uriPrefix ??= call<string>(request, `${FS}.WorkspaceURIPrefix`);
	return `${await uriPrefix}${id}/${path}`;
}

/** Whether the workspace holds an entry at that path, root-relative. */
export async function existsInWorkspace(
	request: APIRequestContext,
	id: string,
	path: string
): Promise<boolean> {
	try {
		await call(request, `${FS}.Stat`, await workspaceURI(request, id, path));
		return true;
	} catch {
		return false;
	}
}

/** Reads a workspace file from disk, through the app's own provider. */
export async function readWorkspaceFile(
	request: APIRequestContext,
	id: string,
	path: string
): Promise<string> {
	return call<string>(request, `${FS}.ReadFile`, { uri: await workspaceURI(request, id, path) });
}

/** The names of the databases the graph is holding, in its own order. */
export async function databasesInGraph(request: APIRequestContext): Promise<string[]> {
	const workspace = await call<{ db_instances: { name: string }[] }>(
		request,
		`${GRAPH}.GetWorkspaceGraph`
	);

	return (workspace.db_instances ?? []).map((db) => db.name);
}

export { expect };
export type { APIRequestContext, Locator, Page };
