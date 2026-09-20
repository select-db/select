import { get, writable } from 'svelte/store';

import { tryCatch } from '$lib/utils/tryCatch';

import type { LoginProvider } from './loginProviders';

export type LoginFlow = {
	provider: LoginProvider;
	/** Still waiting on the provider: neither finished nor failed. */
	running: boolean;
	/** The code the user types at the provider, once there is one. */
	userCode: string;
	verificationUri: string;
	/** When the attempt began, so the countdown survives the modal being closed. */
	startedAt: number;
	error: string | null;
};

/**
 * The sign-in attempt in progress, if any.
 *
 * It lives here rather than in the modal because the modal is a view on it: a
 * person who closes it to reach their browser, or dismisses it by clicking
 * outside, is still signing in, and cancelling the poll under them left the
 * provider authorized and the app on the login screen. Only `cancelLogin` stops
 * it.
 */
export const loginFlowStore = writable<LoginFlow | null>(null);

// Bumped by every start and every cancel, so a superseded attempt cannot write
// its result over the one that replaced it.
let latestAttempt = 0;

function patchFlow(fields: Partial<LoginFlow>) {
	loginFlowStore.update((current) => (current ? { ...current, ...fields } : current));
}

/**
 * Starts signing in with `provider`, or leaves a running attempt alone:
 * reopening the modal reattaches to the code already on screen instead of
 * asking the provider for a second one.
 */
export async function startLogin(provider: LoginProvider): Promise<void> {
	const current = get(loginFlowStore);
	if (current?.running) {
		if (current.provider.id === provider.id) return;
		// Switching methods mid-attempt: the old poll has to be told.
		cancelLogin();
	}

	const thisAttempt = ++latestAttempt;
	const superseded = () => thisAttempt !== latestAttempt;

	loginFlowStore.set({
		provider,
		running: true,
		userCode: '',
		verificationUri: '',
		startedAt: Date.now(),
		error: null
	});

	const [codes, codeErr] = await tryCatch(provider.getDeviceCode);
	if (superseded()) return;
	if (codeErr || !codes) {
		patchFlow({ running: false, error: `Failed to initiate ${provider.name} login.` });
		return;
	}

	patchFlow({ userCode: codes.user_code, verificationUri: codes.verification_uri });

	const [, pollErr] = await tryCatch(provider.startPolling, codes.device_code);
	if (superseded()) return;
	if (pollErr) {
		provider.cancelPolling();
		const reason = pollErr.message ? `: ${pollErr.message}` : '';
		patchFlow({ running: false, error: `Authorization failed or timed out${reason}` });
		return;
	}

	// Signed in. The code stays on screen until the session wall closes the modal,
	// which it does once the workbench is ready.
	patchFlow({ running: false });
}

/** Stops the attempt for good. Closing the modal does not do this. */
export function cancelLogin(): void {
	latestAttempt++;
	const current = get(loginFlowStore);
	if (!current) return;
	current.provider.cancelPolling();
	loginFlowStore.set(null);
}
