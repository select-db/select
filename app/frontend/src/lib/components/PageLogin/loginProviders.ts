import {
	GetAccessToken,
	GetDeviceCode,
	CancelAccessTokenPolling
} from '$lib/bindings/selectDb/internal/auth/githubauth';
import type { DeviceCodeResponse } from '$lib/bindings/selectDb/internal/auth/models';
import type { Icons } from '$lib/system/Icon/types';

/**
 * One way to sign in. Everything the login screen and its modal need comes from
 * here, which is what lets them stay provider-agnostic.
 */
export type LoginProvider = {
	id: string;
	/** Named in the button, the modal title and the flow's error messages. */
	name: string;
	icon: Icons;
	getDeviceCode: () => Promise<DeviceCodeResponse | null>;
	/** Resolves once the provider has authorized; the Go side emits "login". */
	startPolling: (deviceCode: string) => Promise<void>;
	cancelPolling: () => void;
};

/**
 * Not the extension point on its own: a second method also needs its own bound
 * Go service, since the poll loop and the session install it ends with live in
 * GithubAuth. This is the part of it the UI reads.
 */
export const loginProviders: LoginProvider[] = [
	{
		id: 'github',
		name: 'GitHub',
		icon: 'github',
		getDeviceCode: GetDeviceCode,
		startPolling: GetAccessToken,
		cancelPolling: CancelAccessTokenPolling
	}
];
