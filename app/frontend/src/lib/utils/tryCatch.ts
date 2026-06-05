import { notifyError } from '$lib/system/Notifications/notificationsStore';

export type Result<T> = [T, null] | [null, Error];

/** Async overload: returns Promise<Result<T>>. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function tryCatch<T, Args extends any[]>(
	fn: (...args: Args) => Promise<T>,
	...args: Args
): Promise<Result<T>>;

/** Sync overload: returns Result<T>. */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function tryCatch<T, Args extends any[]>(
	fn: (...args: Args) => T,
	...args: Args
): Result<T>;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function tryCatch<T, Args extends any[]>(
	fn: (...args: Args) => T | Promise<T>,
	...args: Args
): Result<T> | Promise<Result<T>> {
	const fnName = fn.name || 'anonymous';
	try {
		const result = fn(...args);
		if (result instanceof Promise) {
			return result
				.then((data) => [data, null] as Result<T>)
				.catch((err) => {
					return [
						null,
						new Error(`${fnName}:${err instanceof Error ? err.message : String(err)}`)
					] as Result<T>;
				});
		}
		return [result, null];
	} catch (err) {
		return [null, new Error(`${fnName}:${err instanceof Error ? err.message : String(err)}`)];
	}
}

export async function must<T>(
	result: Result<T> | Promise<Result<T>>,
	debug?: string | null,
	onError?: () => void
): Promise<T> {
	const start = debug ? performance.now() : 0;

	const [res, err] = await Promise.resolve(result);

	if (debug) {
		const duration = performance.now() - start;
		console.error(`[${debug}] result:`, res);
		console.error(`[${debug}] error:`, err);
		console.error(`[${debug}] execution time: ${duration.toFixed(2)}ms`);
	}

	if (err) {
		onError?.();
		notifyError(err?.message);
		throw err;
	}

	return res;
}
