/**
 * Thin wrapper over the Wails v3 event API.
 *
 * Wails v3 hands listeners a `WailsEvent` object rather than the payload the
 * Go side emitted, and every event we emit carries a single payload. Unwrapping
 * `event.data` here keeps call sites reading `(payload) => ...` and gives them
 * a typed payload.
 */
import { Events } from '@wailsio/runtime';

/** Subscribes to an event. Returns a function that removes this listener. */
export function EventsOn<T = unknown>(eventName: string, callback: (data: T) => void): () => void {
	return Events.On(eventName, (event) => callback(event.data as T));
}

/** Removes every listener registered for the given events. */
export function EventsOff(eventName: string, ...additionalEventNames: string[]): void {
	Events.Off(eventName, ...additionalEventNames);
}

/**
 * Emits an event. The event round-trips through the Go side, which broadcasts
 * it back to every listener, frontend ones included.
 */
export function EventsEmit(eventName: string, data?: unknown): void {
	void Events.Emit(eventName, data);
}
