<script lang="ts">
	import { format } from 'sql-formatter';

	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import SqlViewer from '$lib/system/SqlViewer/SqlViewer.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import { get } from 'svelte/store';

	import { tryCatch } from '$lib/utils/tryCatch';
	import { highlightJson } from '$lib/components/views/Chat/utils/formatting';

	import { getIcon } from '$lib/components/views/shared/getIcon';
	import FieldIndicators from '$lib/components/views/shared/FieldIndicators.svelte';
	import DatabaseIndicator from '$lib/components/shared/DatabaseIndicator/DatabaseIndicator.svelte';
	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import DatabaseSystemInfo from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';

	type DbItem = {
		id?: string;
		name?: string;
		type?: string;
		path?: string;
		badges?: unknown[];
		metadata?: Record<string, unknown>;
		children?: DbItem[];
	};

	type DatabaseItemProps = {
		item?: DbItem;
	};
	let { item }: DatabaseItemProps = $props();

	const availableValues = $derived(() =>
		Object.entries(item?.metadata ?? {}).filter(
			([key]) =>
				[
					'name',
					'description',
					'id',
					'children',
					'hasindex',
					'isforeignkey',
					'isprimarykey',
					'kind',
					'f',
					'oid'
				].includes(key.toLowerCase()) === false
		)
	);

	const children = $derived(() => item?.children);
	const descriptionValue = $derived(() => {
		const raw = item?.metadata?.description;
		if (typeof raw === 'string') {
			const trimmed = raw.trim();
			return trimmed.length > 0 ? trimmed : null;
		}
		return null;
	});

	type BreadcrumbSegment = {
		label: string;
		path: string;
	};

	const breadcrumb = $derived((): BreadcrumbSegment[] => {
		const path = item?.path;
		if (!path) return [];
		const parts = path
			.split(' / ')
			.map((p) => p.trim())
			.filter(Boolean);

		const segments: BreadcrumbSegment[] = [];
		let current = '';

		for (const part of parts) {
			current = current ? `${current} / ${part}` : part;
			segments.push({ label: part, path: current });
		}

		return segments;
	});

	const breadcrumbSegments = $derived(breadcrumb());

	const findDbItemByPath = (targetPath: string): DbItem | undefined => {
		const workspace = get(workspaceGraphStore);
		const dbs = (workspace?.db_instances ?? []);

		for (const db of dbs) {
			const stack: DbItem[] = [...db.children];
			while (stack.length) {
				const node = stack.pop()!;
				if (node.path === targetPath) return node;
				if (Array.isArray(node.children)) stack.push(...node.children);
			}
		}

		return undefined;
	};

	const openBreadcrumbItem = (segmentPath: string, event: MouseEvent) => {
		event.stopPropagation();
		event.preventDefault();
		const target = findDbItemByPath(segmentPath);
		if (!target) return;

		modalStore.set({
			content: () => DatabaseSystemInfo,
			props: { item: target },
			width: 600
		});
	};

	const getBreadcrumbNode = (segment: BreadcrumbSegment) => {
		return findDbItemByPath(segment.path);
	};

	const getBreadcrumbDatabase = (firstSegment: BreadcrumbSegment) => {
		const workspace = get(workspaceGraphStore);
		const dbs = (workspace?.db_instances ?? []);
		return dbs.find((db) => db.name === firstSegment.label);
	};

	const childrenMenuOptions = $derived(() => {
		const allChildren = (children() ?? []);
		return allChildren.map(
			(child: DbItem): MenuOption => ({
				id: child.id ?? child.name ?? '',
				label: child.name ?? '',
				icon: getIcon(child.type),
				badge: Array.isArray(child.badges) ? String(child.badges[0] ?? '') : undefined,
				// For table columns, provide customData so Menu uses custom content
				...(typeof child.type === 'string' && child.type.startsWith('column:')
					? { customData: child }
					: {}),
				action: () =>
					modalStore.set({
						content: () => DatabaseSystemInfo,
						props: { item: child },
						width: 600
					})
			})
		);
	});

	const formatSql = (sql: string): string => {
		// TODO: set the correct format language
		const [formattedSql] = tryCatch(() => format(sql, { language: 'sqlite' })) as [
			string | null,
			Error | null
		];
		return formattedSql ?? sql;
	};

	const isObject = (value: unknown): boolean =>
		value !== null && typeof value === 'object' && !Array.isArray(value);

	const hasContent = $derived(
		!!item &&
			(!!descriptionValue() || availableValues().length > 0 || (children()?.length ?? 0) > 0)
	);
</script>

{#if breadcrumbSegments.length}
	<div class="breadcrumb">
		{#each breadcrumbSegments as segment, i (segment.path)}
			{@const node = getBreadcrumbNode(segment)}
			{#if i > 0}
				<span class="breadcrumb-separator">/</span>
			{/if}
			<!-- svelte-ignore a11y_click_events_have_key_events -->
			<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
			<p
				class="breadcrumb-segment"
				class:current={i === 0 || i === breadcrumbSegments.length - 1}
				onclick={(event) => openBreadcrumbItem(segment.path, event)}
			>
				{#if i === 0}
					{@const db = getBreadcrumbDatabase(segment)}
					<DatabaseIndicator id={db!.id} size={16} loaderSize={14} />
				{:else if node}
					{@const icon = getIcon(node.type)}
					{#if icon}
						<Icon {icon} size={16} />
					{/if}
				{/if}
				<span>{segment.label}</span>
			</p>
		{/each}

		{#if item?.type?.startsWith('column:')}
			<div class="indicators-wrapper">
				<FieldIndicators {item} active={false} />
			</div>
		{/if}
	</div>
{/if}

<ModalBody style="padding: 0;">
	{#if hasContent}
		{#if descriptionValue()}
			<div class="description-block">
				<p class="description-text break">{descriptionValue()}</p>
			</div>
		{/if}
		<div class="table-container scrollable">
			<table class="data-table selectable">
				<tbody>
					{#each availableValues() as [key, value] (key)}
						{#if key === 'sql'}
							<tr class="sql-row">
								<td colspan="2" class="sql-cell">
									<div class="sql-viewer-container">
										<SqlViewer sql={formatSql(value as string)} />
									</div>
								</td>
							</tr>
						{:else}
							<tr>
								<td><p>{key}</p></td>
								{#if isObject(value)}
									<td class="json-cell">
										<pre class="json-pre"><code class="hljs scrollable selectable"
												>{highlightJson(value)}</code
											></pre>
									</td>
								{:else}
									<td>
										<p>{value}</p>
									</td>
								{/if}
							</tr>
						{/if}
					{/each}

					{#if children()?.length}
						{#key item?.id ?? ''}
							<tr>
								<td><p class="children-label">children</p></td>
								<td class="children-cell">
									<Menu
										options={childrenMenuOptions()}
										searchEnabled
										searchPlaceholder="Search..."
										maxHeight={300}
										width="100%"
										noBorder
										optionContent={columnOptionContent}
									/>
								</td>
							</tr>
						{/key}
					{/if}
				</tbody>
			</table>
		</div>
	{:else}
		<div class="empty-state" aria-live="polite">
			<Icon icon="info" size={24} />
			<p>No details to display for this item.</p>
		</div>
	{/if}
</ModalBody>

{#snippet columnOptionContent(child: unknown)}
	{#if child}
		{@const node = child as Record<string, unknown>}
		<div class="column-option">
			<div class="indicators-wrapper">
				<FieldIndicators item={node} />
			</div>
			<Icon icon={getIcon(String(node.type ?? '')) ?? 'text'} size={14} />
			<span class="column-option-label">{String(node.name ?? '')}</span>
		</div>
	{:else}
		<span></span>
	{/if}
{/snippet}

<style>
	.table-container {
		overflow-x: hidden;
		overflow-y: auto;
		max-width: 100%;

		max-height: 450px;
	}

	.description-block {
		padding: var(--space-sm-md);
		border-bottom: var(--border);
	}

	.description-text {
		color: var(--gray-800);
		font-style: italic;
	}

	.data-table {
		width: 100%;
		border-collapse: collapse;
		table-layout: auto;
		border: none;
		overflow: scroll;
	}

	td {
		border: var(--border);
		border-left: none;
		border-top: none;
		padding: var(--space-sm);
		text-align: left;
		vertical-align: top;
	}

	tr td:last-child {
		border-right: none;
	}
	tbody tr:last-child td {
		border-bottom: none;
	}

	td:first-child {
		position: sticky;
		left: 0;
		z-index: 1;
		white-space: nowrap;
		width: 1%;
		max-width: 200px;
		color: var(--gray-800);
		background-color: var(--gray-200);
	}

	td:first-child p {
		color: var(--gray-800);
	}

	td:last-child {
		width: 100%;
		word-break: break-word;
		overflow-wrap: break-word;
	}

	td p {
		margin: 0;
		white-space: pre-wrap;
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		color: var(--gray-900);
	}

	.children-cell {
		padding: 0;
	}

	.children-label {
		padding-top: 4px;
	}

	.json-cell {
		padding: 0 !important;
	}

	.json-pre {
		margin: 0;
		padding: 0;
		font-family: ui-monospace, 'SF Mono', Menlo, Monaco, monospace;
		font-size: 0.85em;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		tab-size: 2;
	}

	.json-pre code,
	.json-pre code.hljs {
		font-family: inherit;
		font-size: inherit;
		background: transparent !important;
		padding: var(--space-sm);
		display: block;
		color: var(--gray-900);
	}

	.json-pre :global(.hljs-string) {
		color: var(--gray-1000);
	}

	.json-pre :global(.hljs-number) {
		color: var(--blue);
	}

	.json-pre :global(.hljs-literal) {
		color: var(--purple);
	}

	.json-pre :global(.hljs-attr),
	.json-pre :global(.hljs-attribute) {
		color: var(--purple);
	}

	.json-pre :global(.hljs-punctuation) {
		color: var(--gray-800);
	}

	.sql-row .sql-cell {
		padding: 0;
	}
	.sql-viewer-container {
		height: 280px;
	}

	.breadcrumb {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
		padding: var(--space-sm-md);
		background-color: var(--gray-200);
		border-bottom: var(--border);
	}

	.breadcrumb-separator {
		color: var(--gray-800);
	}

	.breadcrumb-segment {
		display: inline-flex;
		align-items: center;
		gap: var(--space-xs);

		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.breadcrumb-segment span {
		color: var(--gray-800);
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.breadcrumb-segment:hover span {
		color: var(--gray-1000);
	}
	:global(.breadcrumb-segment:hover svg) {
		stroke: var(--gray-1000) !important;
	}
	.breadcrumb-segment.current span {
		color: var(--gray-900) !important;
	}
	:global(.breadcrumb-segment.current svg) {
		stroke: var(--gray-900) !important;
	}

	.column-option {
		position: relative;
		display: flex;
		align-items: center;
		gap: var(--space-xs);

		width: 100%;
		height: 24px;
		margin-left: 65px;
		padding-left: var(--space-sm);
	}

	.column-option-label {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: var(--fs-sm);
	}

	.breadcrumb .indicators-wrapper {
		margin-left: auto;
		height: 16px;
		display: flex;
		align-items: center;
	}

	.column-option .indicators-wrapper {
		position: absolute;
		right: 100%;
		top: 50%;
		transform: translateY(-50%);
	}

	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-sm);
		padding: var(--space-xl);
		color: var(--gray-800);
		text-align: center;
		min-height: 120px;
	}

	.empty-state p {
		margin: 0;
		font-size: var(--fs-sm);
	}
</style>
