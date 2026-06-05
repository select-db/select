import { cancelQuery } from '$lib/utils/query/useQuery';
import { must, tryCatch } from '$lib/utils/tryCatch';

const SEP = '\0';

function key(dbInstanceId: string, fileId: string): string {
	return `${dbInstanceId}${SEP}${fileId}`;
}

export type QueryRegistry = {
	add(dbInstanceId: string, fileId: string): void;
	remove(dbInstanceId: string, fileId: string): void;
	getAll(): Array<{ dbInstanceId: string; fileId: string }>;
	cancelAll(): Promise<void>;
};

export function createQueryRegistry(): QueryRegistry {
	const set = new Set<string>();

	return {
		add(dbInstanceId: string, fileId: string) {
			set.add(key(dbInstanceId, fileId));
		},
		remove(dbInstanceId: string, fileId: string) {
			set.delete(key(dbInstanceId, fileId));
		},
		getAll() {
			return Array.from(set).map((k) => {
				const i = k.indexOf(SEP);
				return {
					dbInstanceId: k.slice(0, i),
					fileId: k.slice(i + 1)
				};
			});
		},
		async cancelAll() {
			const entries = Array.from(set);
			set.clear();
			for (const k of entries) {
				const i = k.indexOf(SEP);
				await must(
					tryCatch(cancelQuery, {
						DbInstanceID: k.slice(0, i),
						FileID: k.slice(i + 1)
					})
				);
			}
		}
	};
}
