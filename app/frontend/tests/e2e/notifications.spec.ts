import { expect, test } from '@playwright/test';
import { get } from 'svelte/store';

import { AlertType } from '../../src/lib/system/Alert/types';
import { notificationStore, notify } from '../../src/lib/system/Notifications/notificationsStore';

/** Pure test of the notification store, riding the existing runner. */

test.beforeEach(() => notificationStore.set([]));

test('an identical alert replaces the one on screen instead of stacking', () => {
	// Several databases refusing the same way is one problem, not five.
	for (let i = 0; i < 5; i++) {
		notify({ type: AlertType.Error, message: 'pq: sorry, too many clients already' });
	}

	const shown = get(notificationStore);
	expect(shown).toHaveLength(1);
	expect(shown[0].message).toBe('pq: sorry, too many clients already');
});

test('different messages still stack', () => {
	notify({ type: AlertType.Error, message: 'too many clients' });
	notify({ type: AlertType.Error, message: 'permission denied' });

	expect(get(notificationStore).map((n) => n.message)).toEqual([
		'too many clients',
		'permission denied'
	]);
});
