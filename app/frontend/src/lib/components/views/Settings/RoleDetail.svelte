<script lang="ts">
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import { must, tryCatch } from '$lib/utils/tryCatch';
	import { ListPermissions, AddPermission, RemovePermission } from '$lib/wailsjs/go/role/Role';
	import { EventsOn, EventsOff } from '$lib/wailsjs/runtime/runtime';
	import DatabaseIndicator from '$lib/components/shared/DatabaseIndicator/DatabaseIndicator.svelte';
	import type { Component } from 'svelte';
	import type { graph } from '$lib/wailsjs/go/models';
	import { debounce } from '$lib/utils/debounce';
	import {
		resolve,
		buildPermissionMap,
		computePermissionChange,
		permissionKey
	} from './permissions';
	import type { Permission, PermissionChange } from './permissions';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import PermissionInfoModal from './PermissionInfoModal.svelte';
	import { permissionActions, type PermissionActions } from '$lib/stores/myPermissionsStore';

	type SavedState = { expandedKeys?: string[]; scrollTop?: number; search?: string };
	type Props = {
		roleId: string;
		savedState?: SavedState;
		onStateChange?: (state: SavedState) => void;
	};
	let { roleId, savedState, onStateChange }: Props = $props();

	const dbPermissionActions = permissionActions.slice(1); // no manage for schema/table/column
	const columnPermissionActions = new Set<PermissionActions>(['select', 'see', 'update']);

	const APP_ROWS: { label: string; action: string; description: string }[] = [
		{
			label: 'Workspace settings',
			action: 'workspace/settings.write',
			description: 'Allows editing workspace name, git remote, and other workspace-level settings.'
		},
		{
			label: 'Workspace users',
			action: 'workspace/users.manage',
			description: 'Allows inviting, removing, and managing workspace members.'
		},
		{
			label: 'Workspace roles',
			action: 'workspace/roles.manage',
			description: 'Allows creating, editing, and deleting roles and their permission sets.'
		},
		{
			label: 'Workspace API keys',
			action: 'workspace/api-keys.manage',
			description: 'Allows creating, rotating, and revoking API keys for automated clients.'
		}
	];

	function openPermInfo(row: (typeof APP_ROWS)[number]) {
		modalStore.set({
			content: () => PermissionInfoModal as Component,
			width: 280,
			props: { title: row.label, description: row.description }
		});
	}

	let permissions = $state<Permission[]>([]);
	let permissionMap = $derived.by(() => buildPermissionMap(permissions));
	let expanded = $state(new Set<string>(savedState?.expandedKeys ?? []));

	function dbSchemas(db: graph.DBInstanceNode) {
		return (db.children ?? []).filter(Boolean).filter((n) => n.type === 'schema');
	}

	function schemaTables(schema: graph.DBInstanceItemNode) {
		return schema.children
			.filter((n) => n.type === 'tables' || n.type === 'views')
			.flatMap((g) => g.children)
			.filter(Boolean);
	}

	function tableColumns(table: graph.DBInstanceItemNode) {
		const direct = (table.children ?? []).filter(Boolean);
		// columns may be direct children (type==='column') or nested under a 'columns' group node
		const group = direct.find((n) => n.type === 'columns');
		return group
			? (group.children ?? []).filter(Boolean)
			: direct.filter((n) => n.type === 'column');
	}

	let indeterminateMap = $derived.by(() => {
		const m = new Map<string, boolean>();
		for (const db of allDbInstances) {
			for (const action of permissionActions) {
				const dbAllowed = resolve(permissionMap, db.id, '*', '*', '*', action) === 'allow';
				m.set(
					`${db.id}|${action}`,
					!dbAllowed &&
						dbSchemas(db).some(
							(s) =>
								resolve(permissionMap, db.id, s.name, '*', '*', action) === 'allow' ||
								schemaTables(s).some(
									(t) =>
										resolve(permissionMap, db.id, s.name, t.name, '*', action) === 'allow' ||
										tableColumns(t).some(
											(c) =>
												resolve(permissionMap, db.id, s.name, t.name, c.name, action) === 'allow'
										)
								)
						)
				);
			}
			for (const schema of dbSchemas(db)) {
				for (const a of dbPermissionActions) {
					const schemaAllowed = resolve(permissionMap, db.id, schema.name, '*', '*', a) === 'allow';
					m.set(
						`${db.id}|${schema.name}|${a}`,
						!schemaAllowed &&
							schemaTables(schema).some(
								(t) =>
									resolve(permissionMap, db.id, schema.name, t.name, '*', a) === 'allow' ||
									tableColumns(t).some(
										(c) => resolve(permissionMap, db.id, schema.name, t.name, c.name, a) === 'allow'
									)
							)
					);
				}
				for (const table of schemaTables(schema)) {
					for (const action of dbPermissionActions) {
						const tableAllowed =
							resolve(permissionMap, db.id, schema.name, table.name, '*', action) === 'allow';
						m.set(
							`${db.id}|${schema.name}|${table.name}|${action}`,
							!tableAllowed &&
								tableColumns(table).some(
									(c) =>
										resolve(permissionMap, db.id, schema.name, table.name, c.name, action) ===
										'allow'
								)
						);
					}
				}
			}
		}
		return m;
	});

	async function removeMany(toRemove: Permission[]) {
		if (!toRemove.length) return;
		for (const p of toRemove) await must(tryCatch(RemovePermission, p.id));
		const removedIds = new Set(toRemove.map((p) => p.id));
		permissions = permissions.filter((p) => !removedIds.has(p.id));
	}

	async function addPerm(
		db: string,
		schema: string,
		table: string,
		col: string,
		action: string,
		effect: 'allow' | 'deny'
	) {
		const row = await must(
			tryCatch(AddPermission, {
				role_id: roleId,
				db_instance_id: db || undefined,
				schema_name: schema || undefined,
				table_name: table || undefined,
				column_name: col || undefined,
				action,
				effect
			})
		);
		permissions = [...permissions, row as Permission];
	}

	let saving = false;

	async function applyChanges(changes: PermissionChange[]) {
		saving = true;
		try {
			for (const c of changes) {
				if (c.type === 'remove') {
					await must(tryCatch(RemovePermission, c.id));
					permissions = permissions.filter((p) => p.id !== c.id);
				} else {
					await addPerm(c.db, c.schema, c.table, c.col, c.action, c.effect);
				}
			}
		} finally {
			saving = false;
		}
	}

	function toggleExpand(key: string) {
		const next = new Set(expanded);
		if (next.has(key)) next.delete(key);
		else next.add(key);
		expanded = next;
		onStateChange?.({
			expandedKeys: [...next],
			scrollTop: tableWrapEl?.scrollTop,
			search: currentSearch
		});

		// recompute sticky column widths after DOM settles
		requestAnimationFrame(() => updateLayout?.());
	}

	let visibleIds = $state<Set<string> | null>(null); // null = show everything
	let currentSearch = $state(savedState?.search ?? '');

	const applySearch = debounce((v: string) => {
		currentSearch = v;
		onStateChange?.({ expandedKeys: [...expanded], scrollTop: tableWrapEl?.scrollTop, search: v });
		const q = v.trim().toLowerCase();
		if (!q) {
			visibleIds = null;
			return;
		}
		const ids = new Set<string>();
		const toExpand = new Set<string>();
		for (const db of allDbInstances) {
			if (db.name.toLowerCase().includes(q)) ids.add(db.id);
			for (const schema of dbSchemas(db)) {
				if (schema.name.toLowerCase().includes(q)) {
					ids.add(db.id);
					ids.add(schema.id);
					toExpand.add(db.id);
				}
				for (const table of schemaTables(schema)) {
					if (table.name.toLowerCase().includes(q)) {
						ids.add(db.id);
						ids.add(schema.id);
						ids.add(table.id);
						toExpand.add(db.id);
						toExpand.add(`${db.id}|${schema.name}`);
					}
				}
			}
		}
		visibleIds = ids;
		// auto-expand parents of matches, preserving any existing expansions
		expanded = new Set([...expanded, ...toExpand]);
		requestAnimationFrame(() => updateLayout?.());
	}, 500);

	async function grantAllDb(dbId: string) {
		for (const a of permissionActions) {
			if (resolve(permissionMap, dbId, '*', '*', '*', a) === 'allow') continue;
			await applyChanges(computePermissionChange(permissionMap, dbId, '*', '*', '*', a));
		}
	}
	async function revokeAllDb(dbId: string) {
		await removeMany(permissions.filter((p) => p.db_instance_id === dbId));
	}
	async function grantAllSchema(dbId: string, schemaName: string) {
		for (const a of dbPermissionActions) {
			if (resolve(permissionMap, dbId, schemaName, '*', '*', a) === 'allow') continue;
			await applyChanges(computePermissionChange(permissionMap, dbId, schemaName, '*', '*', a));
		}
	}
	async function revokeAllSchema(dbId: string, schemaName: string) {
		await removeMany(
			permissions.filter((p) => p.db_instance_id === dbId && p.schema_name === schemaName)
		);
		// block any inherited access from DB-level wildcard
		for (const a of dbPermissionActions) {
			if (resolve(permissionMap, dbId, schemaName, '*', '*', a) !== 'allow') continue;
			await addPerm(dbId, schemaName, '*', '*', a, 'deny');
		}
	}
	async function grantAllTable(dbId: string, schemaName: string, tableName: string) {
		for (const a of dbPermissionActions) {
			if (resolve(permissionMap, dbId, schemaName, tableName, '*', a) === 'allow') continue;
			await applyChanges(
				computePermissionChange(permissionMap, dbId, schemaName, tableName, '*', a)
			);
		}
	}
	async function revokeAllTable(dbId: string, schemaName: string, tableName: string) {
		await removeMany(
			permissions.filter(
				(p) =>
					p.db_instance_id === dbId && p.schema_name === schemaName && p.table_name === tableName
			)
		);
		// block any inherited access from schema- or DB-level wildcard
		for (const a of dbPermissionActions) {
			if (resolve(permissionMap, dbId, schemaName, tableName, '*', a) !== 'allow') continue;
			await addPerm(dbId, schemaName, tableName, '*', a, 'deny');
		}
	}
	async function grantAllColumn(
		dbId: string,
		schemaName: string,
		tableName: string,
		colName: string
	) {
		for (const a of columnPermissionActions) {
			if (resolve(permissionMap, dbId, schemaName, tableName, colName, a) === 'allow') continue;
			await applyChanges(
				computePermissionChange(permissionMap, dbId, schemaName, tableName, colName, a)
			);
		}
	}
	async function revokeAllColumn(
		dbId: string,
		schemaName: string,
		tableName: string,
		colName: string
	) {
		await removeMany(
			permissions.filter(
				(p) =>
					p.db_instance_id === dbId &&
					p.schema_name === schemaName &&
					p.table_name === tableName &&
					p.column_name === colName
			)
		);
		// block any inherited access from table- or above wildcard
		for (const a of columnPermissionActions) {
			if (resolve(permissionMap, dbId, schemaName, tableName, colName, a) !== 'allow') continue;
			await addPerm(dbId, schemaName, tableName, colName, a, 'deny');
		}
	}

	let allDbInstances = $derived($workspaceGraphStore?.db_instances ?? []);
	let dbInstances = $derived(
		!visibleIds ? allDbInstances : allDbInstances.filter((db) => visibleIds!.has(db.id))
	);

	type DbRow = { type: 'db'; key: string; db: graph.DBInstanceNode; expanded: boolean };
	type SchemaRow = {
		type: 'schema';
		key: string;
		db: graph.DBInstanceNode;
		schema: graph.DBInstanceItemNode;
		expanded: boolean;
	};
	type TableRow = {
		type: 'table';
		key: string;
		db: graph.DBInstanceNode;
		schema: graph.DBInstanceItemNode;
		table: graph.DBInstanceItemNode;
		expanded: boolean;
	};
	type ColRow = {
		type: 'col';
		key: string;
		db: graph.DBInstanceNode;
		schema: graph.DBInstanceItemNode;
		table: graph.DBInstanceItemNode;
		col: graph.DBInstanceItemNode;
	};
	type Row = DbRow | SchemaRow | TableRow | ColRow;

	let visibleRows = $derived.by(() => {
		const rows: Row[] = [];
		for (const db of dbInstances) {
			const dbExpanded = expanded.has(db.id);
			rows.push({ type: 'db', key: db.id, db, expanded: dbExpanded });
			if (!dbExpanded) continue;
			for (const schema of dbSchemas(db).filter((s) => !visibleIds || visibleIds.has(s.id))) {
				const schemaKey = `${db.id}|${schema.name}`;
				const schemaExpanded = expanded.has(schemaKey);
				rows.push({ type: 'schema', key: schemaKey, db, schema, expanded: schemaExpanded });
				if (!schemaExpanded) continue;
				for (const table of schemaTables(schema).filter(
					(t) => !visibleIds || visibleIds.has(t.id)
				)) {
					const tableKey = `${db.id}|${schema.name}|${table.name}`;
					const tableExpanded = expanded.has(tableKey);
					rows.push({ type: 'table', key: tableKey, db, schema, table, expanded: tableExpanded });
					if (!tableExpanded) continue;
					for (const col of tableColumns(table)) {
						rows.push({ type: 'col', key: `${tableKey}|${col.name}`, db, schema, table, col });
					}
				}
			}
		}
		return rows;
	});

	async function load() {
		if (saving) return;
		const result = await must(tryCatch(ListPermissions, roleId));
		permissions = (result as Permission[]) ?? [];
	}

	$effect(() => {
		void roleId;
		load();
		if (savedState?.search) applySearch(savedState.search);
	});

	$effect(() => {
		EventsOn('rolesUpdated', load);
		return () => EventsOff('rolesUpdated');
	});

	let tableWrapEl = $state<HTMLDivElement | null>(null);
	let updateLayout: (() => void) | null = null;

	$effect(() => {
		if (!tableWrapEl) return;
		const update = () => {
			const thead = tableWrapEl!.querySelector('thead');
			const firstRow = tableWrapEl!.querySelector('tbody tr');
			const col1 = tableWrapEl!.querySelector('th.col-resource');
			const col2 = tableWrapEl!.querySelector('th:nth-child(2)');
			if (!thead || !firstRow) return;
			tableWrapEl!.style.setProperty('--thead-h', `${thead.getBoundingClientRect().height}px`);
			tableWrapEl!.style.setProperty('--row-h', `${firstRow.getBoundingClientRect().height}px`);
			if (col1)
				tableWrapEl!.style.setProperty('--col-1-w', `${col1.getBoundingClientRect().width}px`);
			if (col2)
				tableWrapEl!.style.setProperty('--col-2-w', `${col2.getBoundingClientRect().width}px`);
		};
		updateLayout = update;
		update();
		if (savedState?.scrollTop) tableWrapEl!.scrollTop = savedState.scrollTop;
		const onScroll = () =>
			onStateChange?.({
				expandedKeys: [...expanded],
				scrollTop: tableWrapEl!.scrollTop,
				search: currentSearch
			});
		tableWrapEl!.addEventListener('scroll', onScroll, { passive: true });
		const ro = new ResizeObserver(update);
		ro.observe(tableWrapEl!);
		return () => {
			ro.disconnect();
			tableWrapEl?.removeEventListener('scroll', onScroll);
			updateLayout = null;
		};
	});
</script>

<div class="role-detail">
	<div class="table-wrap" bind:this={tableWrapEl}>
		<table class="scrollable">
			<thead>
				<tr>
					<th class="col-resource" style="padding: var(--space-xs);">
						<Input
							value={currentSearch}
							oninput={(e) => applySearch((e.target as HTMLInputElement).value)}
							placeholder="Search…"
							emphasis="low"
							size="md"
							clearable={true}
							onclear={() => applySearch('')}
						/>
					</th>
					{#each permissionActions as action (action)}
						<th class="col-action">{action}</th>
					{/each}
					<th class="col-spacer"></th>
				</tr>
			</thead>
			<tbody>
				<!-- App-level rows: only manage column -->
				{#if !visibleIds}
					{#each APP_ROWS as row (row.action)}
						{@const perm = permissionMap.get(permissionKey('', '', '', '', row.action))?.effect}
						<tr>
							<td class="col-resource">
								<span class="label">{row.label}</span>
								<button class="info-btn" onclick={() => openPermInfo(row)}>
									<Icon icon="info" size={12} />
								</button>
							</td>
							<td class="col-action">
								{#key perm}<Checkbox
										size="sm"
										checked={perm === 'allow'}
										denied={perm === 'deny'}
										onchange={() =>
											applyChanges(
												computePermissionChange(permissionMap, '', '', '', '', row.action)
											)}
									/>{/key}
							</td>
							{#each dbPermissionActions as _ (_)}
								<td class="col-action col-na"></td>
							{/each}
							<td class="col-spacer"></td>
						</tr>
					{/each}
				{/if}

				<!-- DB / schema / table / column rows (flat, keyed for future virtualisation) -->
				{#snippet renderRow(row: Row)}
					{#if row.type === 'db'}
						{@const { db, expanded: dbExpanded } = row}
						<tr class="row-db" class:sticky={dbExpanded}>
							<td class="col-resource">
								<div class="row-content">
									<button class="expand-btn" onclick={() => toggleExpand(db.id)}>
										<Icon icon={dbExpanded ? 'chevron-down' : 'chevron-right'} size={14} />
										<DatabaseIndicator id={db.id} size={14} />
										<span class="label">{db.name}</span>
									</button>
									<div class="row-actions">
										<button class="action-btn" onclick={() => grantAllDb(db.id)}>grant all</button>
										<button class="action-btn action-btn-revoke" onclick={() => revokeAllDb(db.id)}
											>revoke all</button
										>
									</div>
								</div>
							</td>
							{#each permissionActions as action (action)}
								{@const explicit = permissionMap.get(
									permissionKey(db.id, '*', '*', '*', action)
								)?.effect}
								<td
									class="col-action"
									onclick={() =>
										applyChanges(
											computePermissionChange(permissionMap, db.id, '*', '*', '*', action)
										)}
								>
									{#key explicit}<Checkbox
											size="sm"
											checked={resolve(permissionMap, db.id, '*', '*', '*', action) === 'allow'}
											denied={explicit === 'deny'}
											indeterminate={indeterminateMap.get(`${db.id}|${action}`) ?? false}
											onchange={() =>
												applyChanges(
													computePermissionChange(permissionMap, db.id, '*', '*', '*', action)
												)}
										/>{/key}
								</td>
							{/each}
							<td class="col-spacer"></td>
						</tr>
					{:else if row.type === 'schema'}
						{@const { db, schema, key: schemaKey, expanded: schemaExpanded } = row}
						<tr class="row-schema" class:sticky={schemaExpanded}>
							<td class="col-resource indent-1">
								<div class="row-content">
									<button class="expand-btn" onclick={() => toggleExpand(schemaKey)}>
										<Icon icon={schemaExpanded ? 'chevron-down' : 'chevron-right'} size={14} />
										<Icon icon="schema" size={16} stroke="var(--gray-700)" />
										<span class="label">{schema.name}</span>
									</button>
									<div class="row-actions">
										<button class="action-btn" onclick={() => grantAllSchema(db.id, schema.name)}
											>grant all</button
										>
										<button
											class="action-btn action-btn-revoke"
											onclick={() => revokeAllSchema(db.id, schema.name)}>revoke all</button
										>
									</div>
								</div>
							</td>
							<!-- no manage for schema -->
							<td class="col-action col-na"></td>
							{#each dbPermissionActions as action (action)}
								{@const explicit = permissionMap.get(
									permissionKey(db.id, schema.name, '*', '*', action)
								)?.effect}
								<td
									class="col-action"
									onclick={() =>
										applyChanges(
											computePermissionChange(permissionMap, db.id, schema.name, '*', '*', action)
										)}
								>
									{#key explicit}<Checkbox
											size="sm"
											checked={resolve(permissionMap, db.id, schema.name, '*', '*', action) ===
												'allow'}
											denied={explicit === 'deny'}
											indeterminate={indeterminateMap.get(`${db.id}|${schema.name}|${action}`) ??
												false}
											onchange={() =>
												applyChanges(
													computePermissionChange(
														permissionMap,
														db.id,
														schema.name,
														'*',
														'*',
														action
													)
												)}
										/>{/key}
								</td>
							{/each}
							<td class="col-spacer"></td>
						</tr>
					{:else if row.type === 'table'}
						{@const { db, schema, table, key: tableKey, expanded: tableExpanded } = row}
						<tr class="row-table" class:sticky={tableExpanded}>
							<td class="col-resource indent-2">
								<div class="row-content">
									<button class="expand-btn" onclick={() => toggleExpand(tableKey)}>
										<Icon icon={tableExpanded ? 'chevron-down' : 'chevron-right'} size={14} />
										<Icon icon="table" size={16} stroke="var(--gray-700)" />
										<span class="label">{table.name}</span>
									</button>
									<div class="row-actions">
										<button
											class="action-btn"
											onclick={() => grantAllTable(db.id, schema.name, table.name)}
											>grant all</button
										>
										<button
											class="action-btn action-btn-revoke"
											onclick={() => revokeAllTable(db.id, schema.name, table.name)}
											>revoke all</button
										>
									</div>
								</div>
							</td>
							<td class="col-action col-na"></td>
							{#each dbPermissionActions as action (action)}
								{@const explicit = permissionMap.get(
									permissionKey(db.id, schema.name, table.name, '*', action)
								)?.effect}
								<td
									class="col-action"
									onclick={() =>
										applyChanges(
											computePermissionChange(
												permissionMap,
												db.id,
												schema.name,
												table.name,
												'*',
												action
											)
										)}
								>
									{#key explicit}<Checkbox
											size="sm"
											checked={resolve(
												permissionMap,
												db.id,
												schema.name,
												table.name,
												'*',
												action
											) === 'allow'}
											denied={explicit === 'deny'}
											indeterminate={indeterminateMap.get(
												`${db.id}|${schema.name}|${table.name}|${action}`
											) ?? false}
											onchange={() =>
												applyChanges(
													computePermissionChange(
														permissionMap,
														db.id,
														schema.name,
														table.name,
														'*',
														action
													)
												)}
										/>{/key}
								</td>
							{/each}
							<td class="col-spacer"></td>
						</tr>
					{:else}
						{@const { db, schema, table, col } = row}
						<tr class="row-col">
							<td class="col-resource indent-3">
								<div class="row-content">
									<span class="label">{col.name}</span>
									<div class="row-actions">
										<button
											class="action-btn"
											onclick={() => grantAllColumn(db.id, schema.name, table.name, col.name)}
											>grant all</button
										>
										<button
											class="action-btn action-btn-revoke"
											onclick={() => revokeAllColumn(db.id, schema.name, table.name, col.name)}
											>revoke all</button
										>
									</div>
								</div>
							</td>
							<td class="col-action col-na"></td>
							{#each dbPermissionActions as action (action)}
								{#if columnPermissionActions.has(action)}
									{@const explicit = permissionMap.get(
										permissionKey(db.id, schema.name, table.name, col.name, action)
									)?.effect}
									<td
										class="col-action"
										onclick={() =>
											applyChanges(
												computePermissionChange(
													permissionMap,
													db.id,
													schema.name,
													table.name,
													col.name,
													action
												)
											)}
									>
										{#key explicit}<Checkbox
												size="sm"
												checked={resolve(
													permissionMap,
													db.id,
													schema.name,
													table.name,
													col.name,
													action
												) === 'allow'}
												denied={explicit === 'deny'}
												onchange={() =>
													applyChanges(
														computePermissionChange(
															permissionMap,
															db.id,
															schema.name,
															table.name,
															col.name,
															action
														)
													)}
											/>{/key}
									</td>
								{:else}
									<td class="col-action col-na"></td>
								{/if}
							{/each}
							<td class="col-spacer"></td>
						</tr>
					{/if}
				{/snippet}
				{#each visibleRows as row (row.key)}
					{@render renderRow(row)}
				{/each}
			</tbody>
		</table>
	</div>
</div>

<style>
	.role-detail {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
		min-width: 0;
		min-height: 0;
	}

	.table-wrap {
		flex: 1;
		min-height: 0;
		border: var(--border);
		border-radius: var(--br-sm);
		/* Bound the height and clip the table's square corners to the radius. */
		overflow: hidden;
		margin: 0 var(--space-sm) var(--space-sm) var(--space-sm);
	}

	table {
		/* Scroll container lives on the table itself; display:block is required
		   for a <table> to actually scroll. height:100% makes it fill the bounded
		   wrapper so it scrolls instead of overflowing. */
		display: block;
		overflow: auto;
		width: 100%;
		height: 100%;
		border-collapse: separate;
		border-spacing: 0;
		font-size: var(--fs-sm);
	}

	th {
		position: sticky;
		top: 0;
		z-index: 3;
		text-align: center;
		font-weight: var(--fw-light);
		color: var(--gray-800);
		padding: var(--space-xs) var(--space-sm);
		background-color: var(--gray-200);
		border-bottom: var(--border);
		white-space: nowrap;
	}

	th.col-resource {
		text-align: left;
		left: 0;
		z-index: 5;
	}

	th:nth-child(2) {
		left: var(--col-1-w, 200px);
		z-index: 5;
		border-right: var(--border);
	}

	td {
		padding: var(--space-sm-md) var(--space-sm-md);
		border-bottom: var(--border);
		vertical-align: middle;
		background-color: var(--gray-200);
	}

	.col-resource {
		position: sticky;
		left: 0;
		z-index: 2;
		text-align: left;
		white-space: nowrap;
		width: 1px;
		min-width: 200px;
	}

	.col-resource .info-btn {
		display: inline-flex;
		align-items: center;
		background: none;
		border: none;
		padding: 0 2px;
		margin-left: var(--space-xs);
		cursor: pointer;
		color: var(--gray-600);
		vertical-align: middle;
		opacity: 0.5;
	}

	.col-resource .info-btn:hover {
		opacity: 1;
		color: var(--gray-900);
	}

	td:nth-child(2) {
		position: sticky;
		left: var(--col-1-w, 200px);
		z-index: 2;
		border-right: var(--border);
	}

	.col-action {
		text-align: center;
		width: 1px;
		min-width: 36px;
		white-space: nowrap;
		text-transform: uppercase;
		font-size: var(--fs-xs);
	}

	.col-spacer {
		width: 100%;
		padding: 0;
		border-bottom: var(--border);
	}

	.label {
		font-size: var(--fs-sm);
		color: var(--gray-900);
	}

	tbody tr:hover td {
		background-color: var(--gray-100);
	}

	.row-db:hover td {
		background-color: var(--gray-100);
	}

	.row-db.sticky td {
		position: sticky;
		top: var(--thead-h, 30px);
		z-index: 3;
	}
	.row-db.sticky td.col-resource,
	.row-db.sticky td:nth-child(2) {
		z-index: 5;
	}

	.row-schema:hover td {
		background-color: var(--gray-100);
	}

	.row-schema.sticky td {
		position: sticky;
		top: calc(var(--thead-h, 30px) + var(--row-h, 30px));
		z-index: 2;
	}
	.row-schema.sticky td.col-resource,
	.row-schema.sticky td:nth-child(2) {
		z-index: 4;
	}

	.row-table:hover td {
		background-color: var(--gray-100);
	}

	.row-table.sticky td {
		position: sticky;
		top: calc(var(--thead-h, 30px) + 2 * var(--row-h, 30px));
		z-index: 1;
	}
	.row-table.sticky td.col-resource,
	.row-table.sticky td:nth-child(2) {
		z-index: 3;
	}

	.row-content {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		min-width: 0;
	}

	.row-actions {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		right: 0;
		flex-shrink: 0;
		background: var(--gray-100);
		visibility: hidden;
		position: absolute;
		padding: var(--space-xs-sm) var(--space-xs);
		box-shadow: var(--shadow) -15px 0px 10px 0px !important;
	}

	tbody tr:hover .row-actions {
		visibility: visible;
	}

	.action-btn {
		font-size: var(--fs-xs);
		color: var(--blue);
		background: none;
		border: none;
		cursor: pointer;
		padding: 0 var(--space-xs);
		white-space: nowrap;
		opacity: 0.8;
	}
	.action-btn:hover {
		opacity: 1;
		text-decoration: underline;
	}
	.action-btn-revoke {
		color: var(--red);
	}

	.expand-btn {
		display: flex;
		align-items: center;
		gap: var(--space-xs);
		background: none;
		border: none;
		cursor: pointer;
		padding: 0;
		color: var(--gray-900);
	}

	.expand-btn:hover .label {
		text-decoration: underline;
	}

	.indent-1 {
		padding-left: calc(var(--space-sm) + 16px);
	}

	.indent-2 {
		padding-left: calc(var(--space-sm) + 32px);
	}

	.indent-3 {
		padding-left: calc(var(--space-lg) + 48px);
	}
</style>
