import { writable } from 'svelte/store';
import { GetCurrentUser, GetCurrentUserAvatar } from '$lib/bindings/selectDb/internal/user/user';
import { tryCatch } from '$lib/utils/tryCatch';

export type CurrentUser = {
	id: string;
	name: string;
	avatar: string;
};

/** The signed-in user, or null when nobody is. Loaded and cleared by the session wall. */
export const currentUserStore = writable<CurrentUser | null>(null);

export async function loadCurrentUser(): Promise<void> {
	const [user, err] = await tryCatch(GetCurrentUser);
	if (err || !user?.id) {
		currentUserStore.set(null);
		return;
	}

	const [avatar] = await tryCatch(GetCurrentUserAvatar, user.id);
	currentUserStore.set({ id: user.id, name: user.name ?? '', avatar: avatar ?? '' });
}

export function clearCurrentUser(): void {
	currentUserStore.set(null);
}
