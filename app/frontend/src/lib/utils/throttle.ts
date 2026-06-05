// eslint-disable-next-line @typescript-eslint/no-explicit-any
export const throttle = <T extends (...args: any[]) => any>(fn: T, delay: number): T => {
	let lastCall = 0;
	let pending: Promise<void> | null = null;

	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	return function (this: any, ...args: Parameters<T>) {
		const now = Date.now();

		const invoke = async () => {
			lastCall = Date.now();
			await fn.apply(this, args);
			pending = null;
		};

		if (!pending && now - lastCall >= delay) {
			pending = invoke();
		}
	} as T;
};
