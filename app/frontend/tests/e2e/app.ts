import { execFileSync, spawn } from 'node:child_process';
import { mkdirSync, rmSync } from 'node:fs';
import { createServer, type Server, type ServerResponse } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

/**
 * One app per worker: its own process, its own port, its own workspace. Sharing
 * one server is what pinned the suite to a single worker -- specs rewrite the
 * workspace they all read, and the Go side broadcasts every event to every
 * page. A copy each removes both rather than scheduling around them.
 */

const BASE_PORT = Number(process.env.E2E_PORT ?? 9346);
const API_BASE_PORT = BASE_PORT + 100;
const HOST = '127.0.0.1';
const SERVER_BIN = '../build/bin/select-server';
const SEED_BIN = '../build/bin/e2eseed';

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const answers = async (url: string) => {
	try {
		return (await fetch(url, { signal: AbortSignal.timeout(1_000) })).ok;
	} catch {
		return false;
	}
};

/**
 * The SELECT backend, as far as the app can tell. It answers the calls the
 * folder flow makes -- creating and deleting a workspace, and the sync that
 * decides whether a workspace is the user's -- and 404s everything else, which
 * is what the app already got from an unreachable server.
 *
 * A sync that returns nothing is the honest answer for these specs: the fixture
 * database already holds what the user has, and the server has nothing to add.
 *
 * The app addresses it as `localhost:<port>`, the one domain shape it talks to
 * over http rather than https.
 */
function startAPI(port: number): Promise<Server> {
	let created = 0;

	const json = (res: ServerResponse, body: unknown) => {
		res.writeHead(200, { 'content-type': 'application/json' });
		res.end(JSON.stringify(body));
	};

	const server = createServer((req, res) => {
		// Deleting a workspace is the server's to confirm; the rest is local.
		if (req.method === 'DELETE' && req.url?.startsWith('/workspaces/')) {
			res.writeHead(204).end();
			return;
		}

		if (req.method !== 'POST') {
			res.writeHead(404).end();
			return;
		}

		let body = '';
		req.on('data', (chunk) => (body += chunk));
		req.on('end', () => {
			if (req.url === '/workspaces') {
				const id = `created-workspace-${++created}`;
				json(res, {
					id,
					workspace_to_user_id: `${id}-member`,
					name: (JSON.parse(body || '{}').name as string) ?? '',
					owner_id: 'e2e-user'
				});
				return;
			}
			if (req.url === '/sync/v1/sync') {
				json(res, {
					confirmed: [],
					restored: [],
					changes: {},
					server_time: new Date().toISOString()
				});
				return;
			}
			res.writeHead(404).end();
		});
	});

	return new Promise((resolve, reject) => {
		server.once('error', reject);
		server.listen(port, HOST, () => resolve(server));
	});
}

export async function startApp(workerIndex: number) {
	// The app keeps its database and config under the OS config dir. A throwaway
	// one per worker is what makes the workspaces independent.
	//
	// A fixed path wiped on the way in, rather than a fresh one each run: the
	// run is as isolated either way, and this leaves the last one to read
	// afterwards instead of a temp directory per worker per run, forever.
	const dataDir = join(tmpdir(), 'select-e2e', String(workerIndex));
	rmSync(dataDir, { recursive: true, force: true });
	mkdirSync(dataDir, { recursive: true });
	const apiPort = API_BASE_PORT + workerIndex;
	const api = await startAPI(apiPort);
	execFileSync(SEED_BIN, [dataDir, `localhost:${apiPort}`], { stdio: 'pipe' });

	const port = BASE_PORT + workerIndex;
	const url = `http://${HOST}:${port}`;

	const server = spawn(SERVER_BIN, {
		env: {
			...process.env,
			WAILS_SERVER_HOST: HOST,
			WAILS_SERVER_PORT: String(port),
			XDG_CONFIG_HOME: dataDir,
			HOME: dataDir
		},
		stdio: 'ignore'
	});

	const stop = async () => {
		if (server.exitCode === null) server.kill();
		await new Promise((resolve) => api.close(resolve));
	};

	for (let waited = 0; !(await answers(`${url}/`)); waited += 50) {
		// The exit is the answer when it comes: waiting it out costs 60s and says
		// nothing about why.
		if (server.exitCode !== null) throw new Error(`select-server exited (${server.exitCode})`);
		if (waited > 60_000) throw new Error(`select-server never served ${url}`);
		await sleep(50);
	}

	return { url, dataDir, stop };
}
