/**
 * Removes `null` from array element types. Nullable values that are not array
 * entries — an absent `ssh` config, an unknown `errorPosition` — are left
 * alone; those nulls mean something.
 */
export type NonNullItems<T> = T extends (infer U)[]
	? NonNullItems<NonNullable<U>>[]
	: T extends object
		? { [K in keyof T]: NonNullItems<T[K]> }
		: T;

/**
 * Keys whose arrays hold values rather than graph structure, and so are left
 * exactly as the backend sent them.
 *
 * `rows` is query result data: a null cell is a SQL NULL. Removing it does not
 * leave a gap — it shifts every later cell in that row one column to the left,
 * so each value ends up rendered under its neighbour's heading.
 */
const DATA_KEYS = new Set(['rows']);

/**
 * Drops null and undefined entries from every array reachable from `value`,
 * in place — which keeps the model classes' prototypes intact.
 *
 * This exists because a Go nil slice marshals as `null` while the generated
 * types promise an array, so a null entry in the graph only ever means "Go had
 * nothing here". That is not true everywhere: see DATA_KEYS.
 */
export function stripNullItems<T>(value: T): NonNullItems<T> {
	strip(value);
	return value as NonNullItems<T>;
}

function strip(value: unknown): void {
	if (Array.isArray(value)) {
		for (let i = value.length - 1; i >= 0; i--) {
			if (value[i] == null) value.splice(i, 1);
			else strip(value[i]);
		}
		return;
	}

	if (value !== null && typeof value === 'object') {
		for (const [key, nested] of Object.entries(value)) {
			if (DATA_KEYS.has(key)) continue;
			strip(nested);
		}
	}
}
