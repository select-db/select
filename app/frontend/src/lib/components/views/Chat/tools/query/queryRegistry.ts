import { cancelQuery } from '$lib/utils/query/useQuery';
import { must, tryCatch } from '$lib/utils/tryCatch';

const SEP = '\0';

function key(datasourceId: string, fileId: string): string {
	return `${datasourceId}${SEP}${fileId}`;
}

export type QueryRegistry = {
	add(datasourceId: string, fileId: string): void;
	remove(datasourceId: string, fileId: string): void;
	getAll(): Array<{ datasourceId: string; fileId: string }>;
	cancelAll(): Promise<void>;
};

export function createQueryRegistry(): QueryRegistry {
	const set = new Set<string>();

	return {
		add(datasourceId: string, fileId: string) {
			set.add(key(datasourceId, fileId));
		},
		remove(datasourceId: string, fileId: string) {
			set.delete(key(datasourceId, fileId));
		},
		getAll() {
			return Array.from(set).map((k) => {
				const i = k.indexOf(SEP);
				return {
					datasourceId: k.slice(0, i),
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
						DatasourceID: k.slice(0, i),
						FileID: k.slice(i + 1)
					})
				);
			}
		}
	};
}
