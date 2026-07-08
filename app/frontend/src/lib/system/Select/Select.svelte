<script lang="ts">
	import Icon from '$lib/system/Icon/Icon.svelte';
	import type { Icons } from '$lib/system/Icon/types';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import type { MenuOption } from '$lib/system/Menu/Menu.types';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import type { MenuOptionContentSnippet } from '$lib/system/Menu/Menu.types';
	import type { SelectOption, SelectOptionGroup, SelectProps } from './Select.types';

	type Props<V = string> = SelectProps<V>;

	let {
		options = [],
		optionGroups,
		value = $bindable(),
		onchange,
		multiple = false,
		optionDisplay,
		summaryDisplay,
		placeholder = 'Select an option',
		searchEnabled = false,
		searchPlaceholder = 'Search...',
		createOptionLabel,
		onCreate,
		canCreate,

		emphasis = 'high',
		leftIcon,
		iconSize = 18,
		iconColor = 'var(--gray-800)',
		isLoading = false,
		loaderSize = 18,
		size = 'sm',
		style,
		width,
		noRadius,
		menuWidth,
		open = $bindable(false)
	}: Props = $props();
	let triggerEl = $state<HTMLDivElement | null>(null);
	let searchQuery = $state('');
	let internalMenuWidth = $state<number>(menuWidth ?? width ?? 200);

	$effect(() => {
		if (!open || !triggerEl || menuWidth) return;
		internalMenuWidth = Math.round(triggerEl.getBoundingClientRect().width);
	});

	const displayLabel = (option: SelectOption | null): string => option?.label ?? '';

	// Flatten groups when optionGroups is used
	const effectiveOptions = $derived(
		optionGroups?.length
			? (optionGroups as SelectOptionGroup[]).flatMap((g) => g.options)
			: (options as SelectOption[])
	);

	// Selected option(s) for display
	const selectedOptionSingle = $derived(
		!multiple && typeof value !== 'undefined'
			? (effectiveOptions.find((o) => (o as SelectOption).value === value) ?? null)
			: null
	);
	const selectedOptionsMulti = $derived(
		multiple && Array.isArray(value)
			? effectiveOptions.filter((o) => value.includes((o as SelectOption).value))
			: []
	);

	const triggerLabel = $derived.by(() => {
		if (multiple) {
			const sel = selectedOptionsMulti as SelectOption[];
			if (sel.length === 0) return placeholder;
			if (sel.length === 1) return displayLabel(sel[0]!);
			return `${sel.length} selected`;
		}
		return selectedOptionSingle ? displayLabel(selectedOptionSingle) : placeholder;
	});

	const menuOptions = $derived.by((): MenuOption<string>[] => {
		const toMenuOption = (opt: SelectOption, multi: boolean): MenuOption<string> => {
			const option = opt as SelectOption;
			const val = option.value;
			const base = {
				id: String(val),
				label: option.label,
				type: 'option' as const,
				...(optionDisplay ? { customData: option } : {})
			};
			if (multi) {
				const selected = Array.isArray(value) ? value : [];
				const checked = selected.includes(val);
				return {
					...base,
					selected: checked,
					checkbox: true,
					checked,
					onCheckClick: () => {
						const next = checked ? selected.filter((v) => v !== val) : [...selected, val];
						value = next as typeof value;
						onchange?.(next);
					},
					action: () => {
						const sel = Array.isArray(value) ? value : [];
						if (sel.includes(val)) {
							const next = sel.filter((v) => v !== val);
							value = next as typeof value;
							onchange?.(next);
						} else {
							value = [val] as typeof value;
							onchange?.([val]);
						}
					}
				};
			}
			return {
				...base,
				selected: value === val,
				action: () => {
					value = val as typeof value;
					onchange?.(val);
					open = false;
				}
			};
		};

		let out: MenuOption<string>[];
		if (optionGroups?.length) {
			out = [];
			for (const group of optionGroups as SelectOptionGroup[]) {
				out.push({
					id: `group-${group.label}`,
					label: group.label,
					type: 'group-label'
				});
				for (const opt of group.options) {
					out.push(toMenuOption(opt, multiple));
				}
			}
		} else {
			out = effectiveOptions.map((opt) => toMenuOption(opt, multiple));
		}

		// Create option when search doesn't match (like workspace / git branch menus)
		const trimmed = searchQuery.trim();
		if (
			searchEnabled &&
			trimmed &&
			createOptionLabel &&
			onCreate &&
			!effectiveOptions.some(
				(o) => String((o as SelectOption).value).toLowerCase() === trimmed.toLowerCase()
			)
		) {
			const validation = canCreate ? canCreate(trimmed) : true;
			if (validation === true) {
				out = [
					...out,
					{
						id: '__create__',
						label: createOptionLabel(trimmed),
						icon: 'plus',
						action: async () => {
							await onCreate(trimmed);
							open = false;
						}
					}
				];
			} else {
				out = [
					...out,
					{
						id: '__create-error__',
						label: validation,
						icon: 'cross',
						disabled: true
					}
				];
			}
		}
		return out;
	});

	function close() {
		open = false;
		searchQuery = '';
	}
</script>

<div class="select-wrapper" style={width ? `width: ${width}px;` : undefined}>
	{#if leftIcon}
		{#if isLoading}
			<Loader size={loaderSize} />
		{:else}
			<Icon icon={leftIcon as Icons} size={iconSize} stroke={iconColor} />
		{/if}
	{/if}
	<div
		class="select-container"
		class:open
		bind:this={triggerEl}
		role="button"
		tabindex="0"
		onclick={() => (open = !open)}
		onkeydown={(e) => {
			if (e.key === 'Enter' || e.key === ' ') {
				e.preventDefault();
				open = !open;
			}
		}}
		aria-haspopup="listbox"
		aria-expanded={open}
	>
		<div class="select-trigger {size} {emphasis}" class:noRadius style={style ?? ''}>
			{#if multiple}
				{#if summaryDisplay && selectedOptionsMulti.length > 1}
					<span class="select-value select-value-snippet">
						{@render summaryDisplay(selectedOptionsMulti as SelectOption[])}
					</span>
				{:else if optionDisplay && selectedOptionsMulti.length > 0}
					<span class="select-value select-value-snippet">
						{@render optionDisplay(selectedOptionsMulti[0] ?? null)}
					</span>
				{:else}
					<span class="select-value">{triggerLabel}</span>
				{/if}
			{:else if optionDisplay && selectedOptionSingle}
				<span class="select-value select-value-snippet">
					{@render optionDisplay(selectedOptionSingle)}
				</span>
			{:else}
				<span class="select-value">{triggerLabel}</span>
			{/if}
			<div class="arrow" class:open>
				<Icon icon="chevron-down" size={17} />
			</div>
		</div>
	</div>
</div>

{#if open && triggerEl}
	<Portal>
		<FloatingBox anchor={triggerEl} offset={{ x: 0, y: 4 }} backdrop onBackdropClick={close}>
			<Menu
				options={menuOptions}
				sortOptions={true}
				onClose={close}
				bind:searchQuery
				{searchEnabled}
				{searchPlaceholder}
				maxHeight={300}
				width={internalMenuWidth}
				highlightActiveOption={true}
				optionContent={optionDisplay as MenuOptionContentSnippet | undefined}
			/>
		</FloatingBox>
	</Portal>
{/if}

<style>
	.select-wrapper {
		position: relative;
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		box-shadow: var(--shadow-subtle);
	}

	.select-trigger {
		width: 100%;
		min-width: 0;
		max-width: 100%;
		box-sizing: border-box;
		padding-right: 20px;
		transition: all 0.2s;
		display: flex;
		align-items: center;
	}
	.select-trigger:not(.low) {
		background-color: var(--gray-200);
		border: var(--border);
	}
	.select-trigger.low {
		border: 0.5px transparent solid;
		background-color: transparent;
	}
	.select-trigger.noRadius {
		border-radius: 0;
	}

	.select-trigger.xs {
		padding: var(--space-xs-sm) var(--space-xs) var(--space-xs-sm) var(--space-sm);
	}
	.select-trigger.xs:not(.noRadius) {
		border-radius: var(--br-sm);
	}
	.select-trigger.sm {
		padding: var(--space-sm);
	}
	.select-trigger.sm:not(.noRadius) {
		border-radius: var(--br-sm);
	}

	.select-container:focus-within .select-trigger,
	.select-container.open .select-trigger,
	.select-trigger:hover {
		background-color: var(--gray-300);
	}
	.select-trigger.low:hover,
	.select-container.open .select-trigger.low {
		border-color: var(--border-color);
	}
	.select-container:focus-within .select-trigger,
	.select-container.open .select-trigger {
		border-color: var(--gray-600);
	}

	/* Low emphasis: ensure active/select state uses contrast-light border color */
	.select-container:focus-within .select-trigger.low,
	.select-container.open .select-trigger.low,
	.select-trigger.low:hover {
		border-color: var(--border-color);
	}

	/* Low emphasis: dimmed text, brighten on hover/focus (applies to all children) */
	:global(.select-trigger p),
	:global(.select-trigger span) {
		color: var(--gray-800);
	}
	:global(.select-container:focus-within .select-trigger p),
	:global(.select-container:focus-within .select-trigger span),
	:global(.select-container.open .select-trigger p),
	:global(.select-container.open .select-trigger span),
	:global(.select-trigger:hover p),
	:global(.select-trigger:hover span) {
		color: var(--gray-1000);
	}

	.select-value {
		flex: 1;
		text-align: left;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.select-value-snippet {
		display: flex;
		align-items: center;
		overflow: hidden;
	}

	.arrow {
		display: flex;
		align-items: center;
		justify-content: center;
		pointer-events: none;
		transform: rotate(0deg);
		transform-origin: center center;
		transition:
			transform 0.15s ease-out,
			opacity 0.1s ease-out;
	}
	.arrow.open {
		transform: rotate(180deg);
	}
	.select-trigger.xs .arrow {
		margin-left: var(--space-xs);
	}
	.select-trigger.sm .arrow {
		margin-left: var(--space-sm);
	}

	/* Low emphasis: hide arrow until hover / focus / menu open */
	.select-trigger.low .arrow {
		opacity: 0;
	}
	.select-container.open .select-trigger.low .arrow,
	.select-container:focus-within .select-trigger.low .arrow,
	.select-trigger.low:hover .arrow {
		opacity: 1;
	}

	.select-container {
		position: relative;
		flex: 1;
		min-width: 0;
	}
</style>
