import { writable } from 'svelte/store';
import { GetServerManifest } from '$lib/bindings/selectDb/internal/server/server';

export type ServerManifest = {
	live: boolean;
	backend_version: string;
	min_app_version: string;
	latest_app_version: string;
	warning: string;
};

export type ServerStatus = 'idle' | 'loading' | 'live' | 'offline';

type DomainState = {
	status: ServerStatus;
	manifest: ServerManifest | null;
};

/** Per-domain status and manifest. Each listed server has its own entry. */
export const serverIndicatorStore = writable<Record<string, DomainState>>({});

export async function fetchServerManifest(domain: string): Promise<void> {
	if (!domain.trim()) return;
	serverIndicatorStore.update((state) => ({
		...state,
		[domain]: { status: 'loading', manifest: null }
	}));
	const manifest = await GetServerManifest(domain);
	serverIndicatorStore.update((state) => ({
		...state,
		[domain]: {
			status: manifest?.live ? 'live' : 'offline',
			manifest: manifest ? (manifest as ServerManifest) : null
		}
	}));
}

/** Fetch manifests for multiple domains in parallel. Used to show status for all options in the server list. */
export async function fetchServerManifests(domains: string[]): Promise<void> {
	const trimmed = domains.filter((d) => d.trim());
	if (trimmed.length === 0) return;
	trimmed.forEach((d) => {
		serverIndicatorStore.update((s) => ({
			...s,
			[d]: { status: 'loading', manifest: null }
		}));
	});
	const results = await Promise.all(trimmed.map((d) => GetServerManifest(d)));
	serverIndicatorStore.update((state) => {
		const next = { ...state };
		results.forEach((manifest, i) => {
			const domain = trimmed[i]!;
			next[domain] = {
				status: manifest?.live ? 'live' : 'offline',
				manifest: manifest ? (manifest as ServerManifest) : null
			};
		});
		return next;
	});
}
