import { execFileSync, spawn } from 'node:child_process';
import { mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

/**
 * One app per worker: its own process, its own port, its own workspace. Sharing
 * one server is what pinned the suite to a single worker -- specs rewrite the
 * workspace they all read, and the Go side broadcasts every event to every
 * page. A copy each removes both rather than scheduling around them.
 */

const BASE_PORT = Number(process.env.E2E_PORT ?? 9346);
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
	execFileSync(SEED_BIN, [dataDir], { stdio: 'pipe' });

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
	};

	for (let waited = 0; !(await answers(`${url}/`)); waited += 50) {
		// The exit is the answer when it comes: waiting it out costs 60s and says
		// nothing about why.
		if (server.exitCode !== null) throw new Error(`select-server exited (${server.exitCode})`);
		if (waited > 60_000) throw new Error(`select-server never served ${url}`);
		await sleep(50);
	}

	return { url, stop };
}
