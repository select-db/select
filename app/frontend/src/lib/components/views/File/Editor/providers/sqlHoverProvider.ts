import * as monaco from 'monaco-editor';
import { Hover } from '$lib/bindings/selectDb/internal/sqllang/sqllang';
import type * as graph from '$lib/wails/graph';

const debounceMs = 220;
const pending = new Map<string, ReturnType<typeof setTimeout>>();
const cache = new Map<string, { markdown: string; at: number }>();
const cacheTtlMs = 4000;

export function createSqlHoverProvider(
	getFile: () => graph.FileNode | null
): monaco.languages.HoverProvider {
	return {
		provideHover: (model, position) => {
			return new Promise((resolve) => {
				const file = getFile();
				const datasourceId = file?.datasources?.[0]?.id;
				if (!file || !datasourceId) {
					resolve(null);
					return;
				}

				const sql = model.getValue();
				const key = `${datasourceId}:${position.lineNumber}:${position.column}:${sql.length}`;
				const hit = cache.get(key);
				if (hit && Date.now() - hit.at < cacheTtlMs) {
					resolve(hit.markdown ? { contents: [{ value: hit.markdown, isTrusted: true }] } : null);
					return;
				}

				const prev = pending.get(key);
				if (prev) clearTimeout(prev);

				pending.set(
					key,
					setTimeout(async () => {
						pending.delete(key);
						try {
							const r = await Hover({
								DatasourceID: datasourceId,
								FileID: file.id,
								SQL: sql,
								Line: position.lineNumber,
								Column: position.column - 1
							});
							const md = r?.markdown?.trim() ?? '';
							cache.set(key, { markdown: md, at: Date.now() });
							resolve(md ? { contents: [{ value: md, isTrusted: true }] } : null);
						} catch {
							cache.set(key, { markdown: '', at: Date.now() });
							resolve(null);
						}
					}, debounceMs)
				);
			});
		}
	};
}
