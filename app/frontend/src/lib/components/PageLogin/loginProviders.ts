import {
	GetAccessToken,
	GetDeviceCode,
	CancelAccessTokenPolling
} from '$lib/bindings/selectDb/internal/auth/githubauth';
import type { Icons } from '$lib/system/Icon/types';

/** What a provider hands back to show the user and to poll with. */
export type DeviceCodeResult = {
	user_code: string;
	device_code: string;
	verification_uri: string;
};

/**
 * One way to sign in. Everything the login screen and its modal need comes from
 * here, so a second method is an entry in `loginProviders` and nothing else.
 */
export type LoginProvider = {
	id: string;
	/** Named in the button, the modal title and the flow's error messages. */
	name: string;
	icon: Icons;
	getDeviceCode: () => Promise<DeviceCodeResult | null>;
	/** Resolves once the provider has authorized; the Go side emits "login". */
	startPolling: (deviceCode: string) => Promise<void>;
	cancelPolling: () => void;
};

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
