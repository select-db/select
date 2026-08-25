import { expect, test } from '@playwright/test';

import { stripNullItems } from '../../src/lib/wails/stripNullItems';

/**
 * A pure test of the graph-boundary transform, riding the existing runner —
 * the module it imports has no runtime dependencies, so no page is involved.
 */

test('a SQL NULL keeps its place in a row', () => {
	// The shape that broke: a nullable column in the middle of a result. Dropping
	// the null slides every later cell one column left, so `occurred_at` renders
	// under `workspace_id` and the last column shows the one after it.
	const result = {
		columns: ['id', 'workspace_id', 'occurred_at', 'domain'],
		rows: [
			['id-1', null, '2026-08-20T13:05:43+02:00', 'auth'],
			['id-2', 'ws-1', '2026-08-20T13:58:27+02:00', 'auth']
		]
	};

	stripNullItems(result);

	expect(result.rows[0]).toEqual(['id-1', null, '2026-08-20T13:05:43+02:00', 'auth']);
	expect(result.rows.every((row) => row.length === result.columns.length)).toBe(true);
});

test('a nil slice from Go is still dropped everywhere else', () => {
	// What the transform is for: the bindings type these as (T | null)[] because
	// JSON could carry a null, and a Go nil slice does.
	const graph = {
		folders: [{ name: 'queries' }, null],
		explain: { children: [null, { children: [null] }] }
	};

	stripNullItems(graph);

	expect(graph.folders).toEqual([{ name: 'queries' }]);
	expect(graph.explain.children).toEqual([{ children: [] }]);
});
