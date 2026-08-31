import * as monaco from 'monaco-editor';
import { Hover } from '$lib/bindings/selectDb/internal/sqllang/sqllang';
import type * as graph from '$lib/wails/graph';

const debounceMs = 220;
const pending = new Map<string, ReturnType<typeof setTimeout>>();
const cache = new Map<string, { markdown: string; at: number }>();
const cacheTtlMs = 4000;

// Hover markdown is built from the connected database's catalog — table, column
// and function names, and their descriptions — so it is no more trustworthy than
// the database the user happens to have opened.
//
// Monaco's `isTrusted` is what decides whether such a string may execute:
// with it set, `command:` links keep a live data-href that the editor's click
// handler runs, and raw <span> tags pass through instead of being dropped
// (markdownRenderer.js:300 and :348). The Go builders in internal/sqllang/hover.go
// emit only bold text, inline code, rules and GFM tables — never a link or raw
// HTML — so leaving it off costs nothing and closes both paths. Verified by
// rendering every hover shape through monaco both ways: byte-identical output.
export function createSqlHoverProvider(
	getFile: () => graph.FileNode | null
): monaco.languages.HoverProvider {
	return {
		provideHover: (model, position) => {
			return new Promise((resolve) => {
				const file = getFile();
				const dbId = file?.databases?.[0]?.id;
				if (!file || !dbId) {
					resolve(null);
					return;
				}

				const sql = model.getValue();
				const key = `${dbId}:${position.lineNumber}:${position.column}:${sql.length}`;
				const hit = cache.get(key);
				if (hit && Date.now() - hit.at < cacheTtlMs) {
					resolve(hit.markdown ? { contents: [{ value: hit.markdown }] } : null);
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
								DbInstanceID: dbId,
								FileID: file.id,
								SQL: sql,
								Line: position.lineNumber,
								Column: position.column - 1
							});
							const md = r?.markdown?.trim() ?? '';
							cache.set(key, { markdown: md, at: Date.now() });
							resolve(md ? { contents: [{ value: md }] } : null);
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
