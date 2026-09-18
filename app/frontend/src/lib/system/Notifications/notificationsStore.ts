import { get, writable } from 'svelte/store';
import { AlertType } from '../Alert/types';

type Notification = {
	id: string;
	type: AlertType;
	message: string;
	duration: number;
	copyable: boolean;
	timeoutId?: ReturnType<typeof setTimeout>;
	remainingTime?: number;
};

type NotificationState = Notification[];

export const notificationStore = writable<NotificationState>([]);

export const notify = ({
	type,
	message,
	duration = 4000,
	copyable = false
}: {
	type: AlertType;
	message: string;
	duration?: number;
	copyable?: boolean;
}) => {
	// An identical alert already on screen is replaced rather than stacked. Now
	// that schema failures always report, a workspace whose databases all fail
	// the same way would otherwise bury the screen in copies of one message.
	const duplicate = get(notificationStore).find((n) => n.type === type && n.message === message);
	if (duplicate) {
		clearTimeout(duplicate.timeoutId);
		notificationStore.update((list) => list.filter((n) => n.id !== duplicate.id));
	}

	const id = crypto.randomUUID();
	const timeoutId = setTimeout(() => {
		notificationStore.update((list) => list.filter((n) => n.id !== id));
	}, duration);
	const notification: Notification = {
		id,
		type,
		message,
		duration,
		copyable,
		timeoutId,
		remainingTime: duration
	};
	notificationStore.update((notifications) => [...notifications, notification]);
};

export const pauseNotification = (id: string) => {
	notificationStore.update((notifications) => {
		const notification = notifications.find((n) => n.id === id);
		if (notification?.timeoutId) {
			clearTimeout(notification.timeoutId);
			notification.timeoutId = undefined;
		}
		return notifications;
	});
};

export const resumeNotification = (id: string) => {
	notificationStore.update((notifications) => {
		const notification = notifications.find((n) => n.id === id);
		if (notification && !notification.timeoutId && notification.remainingTime) {
			const timeoutId = setTimeout(() => {
				notificationStore.update((list) => list.filter((n) => n.id !== id));
			}, notification.remainingTime);
			notification.timeoutId = timeoutId;
		}
		return notifications;
	});
};

export const dismissNotification = (id: string) => {
	notificationStore.update((notifications) => {
		const notification = notifications.find((n) => n.id === id);
		if (notification?.timeoutId) {
			clearTimeout(notification.timeoutId);
		}
		return notifications.filter((n) => n.id !== id);
	});
};

export const notifySuccess = (message: Notification['message']) =>
	notify({
		type: AlertType.Success,
		message
	});

export const notifyError = (input: Notification['message'] | Error) => {
	const message = input instanceof Error ? input.message : input;
	notify({
		type: AlertType.Error,
		message,
		copyable: true
	});
};
