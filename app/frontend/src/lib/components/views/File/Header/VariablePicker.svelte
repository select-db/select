<script lang="ts">
	import * as graphApi from '$lib/bindings/selectDb/internal/graph/graph';
	import { createEventDispatcher } from 'svelte';

	import { tryCatch } from '$lib/utils/tryCatch';

	import Menu from '$lib/system/Menu/Menu.svelte';
	import Button, { type ButtonEmphasis } from '$lib/system/Button/Button.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import { notify, notifyError } from '$lib/system/Notifications/notificationsStore';
	import { AlertType } from '$lib/system/Alert/types';

	type VariablePickerProps = {
		uri: string;
		emphasis?: ButtonEmphasis;
		style?: string;
		iconSize?: number;
		/** When true, show SQL files from the same folder as insertable refs ($name). */
		allowSqlFileRefs?: boolean;
		/** Optional: when provided, called with the selected variable string instead of copying to clipboard. */
		onchange?: (variable: string) => void;
	};

	let {
		uri,
		emphasis,
		style,
		iconSize = 17,
		allowSqlFileRefs = false,
		onchange
	}: VariablePickerProps = $props();

	const dispatch = createEventDispatcher<{ select: string }>();

	let isOpen = $state(false);
	let searchQuery = $state('');
	type EnvVar = { name: string; value: string; source: string };
	type SqlFileRef = { name: string; path: string; uri: string; preview: string };
	let variables = $state<EnvVar[]>([]);
	let sqlFileRefs = $state<SqlFileRef[]>([]);
	let loading = $state(false);
	let buttonElement: HTMLElement | null = $state(null);

	const choose = async (variable: string) => {
		if (onchange) {
			onchange(variable);
		} else {
			await copyToClipboard(variable);
		}
		dispatch('select', variable);
		isOpen = false;
	};

	// Load env variables and SQL file refs when the menu opens
	const loadVariables = async () => {
		loading = true;
		try {
			const varsPromise = tryCatch(graphApi.GetUriVariables, uri).then(([v]) => v ?? []);
			const filesPromise =
				allowSqlFileRefs && typeof graphApi.GetUriSqlFileRefs === 'function'
					? tryCatch(graphApi.GetUriSqlFileRefs, uri).then(([f]) => f ?? [])
					: Promise.resolve([]);
			const [vars, files] = await Promise.all([varsPromise, filesPromise]);
			variables = Array.isArray(vars) ? vars : [];
			sqlFileRefs = Array.isArray(files) ? files : [];
		} catch {
			notifyError('Failed to load variables');
			variables = [];
			sqlFileRefs = [];
		} finally {
			loading = false;
		}
	};

	// Create menu options: group Environment variables, then group SQL files (preview as badge)
	const menuOptions = $derived.by((): MenuOption<string>[] => {
		const options: MenuOption<string>[] = [];

		if (variables.length > 0) {
			options.push({
				id: 'group-env',
				label: 'Environment variables',
				type: 'group-label'
			} as MenuOption<string>);
			for (const variable of variables) {
				options.push({
					id: `env-${variable.name}`,
					label: `$${variable.name}`,
					badge:
						variable.value.length > 30 ? `${variable.value.substring(0, 30)}...` : variable.value,
					metadata: `$${variable.name}`,
					action: async (option) => {
						await choose((option.metadata || option.label) as string);
					}
				});
			}
		}

		if (sqlFileRefs.length > 0) {
			options.push({
				id: 'group-sql',
				label: 'SQL files',
				type: 'group-label'
			} as MenuOption<string>);
			for (const file of sqlFileRefs) {
				options.push({
					id: `file-${file.name}`,
					label: `$${file.name}`,
					badge: file.preview ?? file.path ?? '',
					metadata: `$${file.name}`,
					action: async (option) => {
						await choose((option.metadata || option.label) as string);
					}
				});
			}
		}

		return options;
	});

	const copyToClipboard = async (text: string) => {
		try {
			await navigator.clipboard.writeText(text);
			notify({
				type: AlertType.Success,
				message: `Copied ${text} to clipboard`
			});
		} catch {
			notifyError('Failed to copy to clipboard');
		}
	};

	const toggleMenu = async () => {
		if (!isOpen) {
			await loadVariables();
		}
		isOpen = !isOpen;
		if (isOpen) {
			searchQuery = '';
		}
	};

	const closeMenu = () => {
		isOpen = false;
	};
</script>

<div bind:this={buttonElement}>
	<Button
		label="Variables"
		noCapitalizeTooltip
		leftIcon="code-bracket"
		{iconSize}
		onclick={toggleMenu}
		active={isOpen}
		{emphasis}
		{style}
	/>
</div>

{#if isOpen}
	<FloatingBox anchor={buttonElement} offset={{ x: 0, y: 4 }} backdrop onBackdropClick={closeMenu}>
		<Menu
			options={menuOptions}
			searchEnabled={true}
			searchPlaceholder="Search variables..."
			bind:searchQuery
			onClose={closeMenu}
			{loading}
			emptyMessage="No environment variables or SQL files in this folder"
			noResultsMessage="No matches"
			maxHeight={300}
			width={350}
		/>
	</FloatingBox>
{/if}
