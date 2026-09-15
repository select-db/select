import {
	expect,
	test as base,
	type APIRequestContext,
	type Locator,
	type Page,
	type Route
} from '@playwright/test';
import { testId, treeRow } from './selectors';
import { startApp } from './app';

/**
 * Talking to the Go side over `/wails/runtime`, the endpoint the app's own
 * bindings call, but from the test process rather than from inside the page.
 *
 * Over HTTP rather than by importing the runtime the page is served: that
 * registers a second client id and takes over the event dispatcher, after which
 * the app's own calls come back 422. The cost is naming two protocol constants.
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

export const test = base.extend<
	{
		emit: (name: string, data?: unknown) => Promise<void>;
		signIn: () => Promise<void>;
		consoleErrors: string[];
		/** Where the app keeps its database, its config and its workspace folder. */
		dataDir: string;
	},
	{ app: { url: string; dataDir: string } }
>({
	app: [
		async ({}, use, workerInfo) => {
			const { url, dataDir, stop } = await startApp(workerInfo.workerIndex);
			await use({ url, dataDir });
			await stop();
		},
		{ scope: 'worker' }
	],

	// Carries `page` and `request` with it, so a spec never names a port.
	baseURL: async ({ app }, use) => {
		await use(app.url);
	},

	dataDir: async ({ app }, use) => {
		await use(app.dataDir);
	},

	/**
	 * Emits an event as the backend would, which the Go side broadcasts to every
	 * listener.
	 *
	 * Events are never queued or replayed, and the socket carrying them comes up
	 * independently of the bundle that listens on it, so one sent too early is
	 * dropped with nothing to wait on instead. Callers emit inside a poll until
	 * they see the effect -- every event used here is safe to send twice.
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
	 * `login` is what the session wall listens for, and the Go side emits it on
	 * finding a stored token. Tokens live in the OS keyring, which a headless
	 * runner has none of, so the suite emits it and lets the app read the seeded
	 * database for the rest.
	 *
	 * Emitting immediately rather than waiting for the login wall to paint: the
	 * app listens before it paints, so the wait is a render nothing needs (~960ms
	 * of setup against ~810ms). The tight first intervals are the other half of
	 * that -- the first emit usually lands before anyone is listening, putting
	 * the retry on every test's critical path.
	 */
	signIn: async ({ page, emit }, use) => {
		await use(async () => {
			await expect
				.poll(
					async () => {
						await emit('login');
						return testId(page, 'tree.panel').isVisible();
					},
					{ intervals: [30, 30, 60, 100, 200, 400] }
				)
				.toBe(true);
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
 * Wails' ids for bound methods, from the generated bindings under
 * `src/lib/bindings/`. Derived from each method's qualified name, so they are
 * stable across builds and change only on a rename or a move -- at which point
 * the specs routing them fail. Regenerate the bindings and copy the new id.
 */
const CHECK_FOR_LOGOUT = 2480583021;

/** DbClient.Query, for the specs that hold a query open or answer it themselves. */
export const QUERY_CALL = 2964708639;

/**
 * Keeps the session up. With no keyring to hold a token, the app's 500ms
 * CheckForLogout poll emits `logout` half a second in and everything after runs
 * on a login screen. Answering that one call changes nothing else.
 */
export async function holdSession(page: Page) {
	await intercept(page, CHECK_FOR_LOGOUT, (route) => route.fulfill({ status: 200, body: '' }));
}

/**
 * Answers one bound Go method from the test, leaving every other call alone.
 * Handlers stack: a non-matching call falls back to whatever was registered
 * before it, and to the app itself when nothing was.
 */
export async function intercept(
	page: Page,
	methodID: number,
	handler: (route: Route) => Promise<unknown>
) {
	await page.route('**/wails/runtime', async (route) => {
		if (!(route.request().postData() ?? '').includes(`"methodID":${methodID}`)) {
			await route.fallback();
			return;
		}
		await handler(route);
	});
}

/**
 * For waits on something the app runs a real query for. The default 5s is the
 * right budget for a render; a round trip through SQLite on a loaded CI runner
 * is not a render.
 */
export const AFTER_QUERY = { timeout: 20_000 };

/** Signs in and waits for the seeded workspace: the state every spec starts from. */
export async function open(page: Page, signIn: () => Promise<void>) {
	await holdSession(page);
	await page.goto('/');
	await signIn();
	await expect(treeRow(page, 'weekly_revenue.sql')).toBeVisible();
}

/** Qualified Go names, which is what the runtime dispatches on. */
const FS = 'selectDb/internal/fs_provider.FSProvider';
export const GRAPH = 'selectDb/internal/graph.Graph';

/** The id of the workspace the seed left, which every path below hangs off. */
export async function workspaceId(request: APIRequestContext): Promise<string> {
	const workspace = await call<{ id: string }>(request, `${GRAPH}.GetWorkspaceGraph`);
	return workspace.id;
}

/** Long enough to outlast the app's debounced git status, short enough to fail fast. */
const GIT_LOCK_RETRIES = 20;
const GIT_LOCK_RETRY_MS = 100;

/**
 * Runs a command in the workspace root, standing in for everything that changes
 * a workspace without going through the app: a terminal, a git checkout, an
 * editor somebody else has open. The app only finds out by watching.
 *
 * Retried on .git/index.lock. Each change here wakes the watcher, which runs a
 * git of the app's own ~200ms later, and two gits in one repository contend for
 * that lock. The lock is held for one command rather than for anything a test
 * could wait on, so retrying is what git itself prescribes -- and it is the same
 * race a person hits running git beside the open app, so failing here would be
 * asserting something that is not true.
 */
export async function exec(
	request: APIRequestContext,
	id: string,
	command: string,
	...args: string[]
) {
	for (let attempt = 0; ; attempt++) {
		const result = await call<{ exitCode: number; stderr: string }>(
			request,
			`${FS}.ExecuteCommand`,
			{ workspaceId: id, command, args }
		);
		if (result.exitCode === 0) return;

		if (attempt < GIT_LOCK_RETRIES && result.stderr.includes('index.lock')) {
			await new Promise((resolve) => setTimeout(resolve, GIT_LOCK_RETRY_MS));
			continue;
		}

		throw new Error(`${command} ${args.join(' ')}: ${result.stderr}`);
	}
}

/** Fixed for the whole run, and asked for inside polls, so fetched once. */
let uriPrefix: Promise<string> | null = null;

async function workspaceURI(request: APIRequestContext, id: string, path: string) {
	uriPrefix ??= call<string>(request, `${FS}.WorkspaceURIPrefix`);
	return `${await uriPrefix}${id}/${path}`;
}

/** Whether the workspace holds an entry at that path, root-relative. */
export async function onDisk(
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
export async function readFile(
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
export type { APIRequestContext, Locator, Page, Route };
