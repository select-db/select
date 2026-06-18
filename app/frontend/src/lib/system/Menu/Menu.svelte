<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import Icon from '$lib/system/Icon/Icon.svelte';
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';
	import type { MenuOption, MenuProps } from './Menu.types';
	import { setContext, getContext } from '$lib/stores/keybindingsContextStore';
	import { registerCommand, unregisterCommand } from '$lib/stores/commandRegistry';
	import { modalStore } from '$lib/system/Modal/ModalStore';

	type T = string;

	let {
		options = [],
		sortOptions = false,
		searchEnabled = false,
		searchPlaceholder = 'Search...',
		searchQuery = $bindable(''),
		externalFilter = false,
		searchValidator,
		loading = false,
		emptyMessage = 'No options available',
		noResultsMessage = 'No results found',
		onClose,
		maxHeight,
		minHeight,
		width = '300px',
		style = '',
		highlightActiveOption = true,
		optionContent,
		noBorder = false
	}: MenuProps<T> = $props();

	let activeIndex = $state(-1); // -1 means search input is active, 0+ means option index
	let pendingOptionId = $state<string | null>(null);
	let mouseInteractionEnabled = $state(false);
	let mouseMoveHandler: ((event: MouseEvent) => void) | null = null;
	let containerRef: HTMLDivElement;
	let searchInputWrapperRef: HTMLDivElement | undefined;
	const sortedOptions = $derived.by(() => {
		const hasGroups = options.some((opt) => opt.type === 'group-label' || opt.type === 'divider');
		if (sortOptions && !hasGroups) {
			return [...options].sort((a, b) =>
				a.label.localeCompare(b.label, undefined, { sensitivity: 'base' })
			);
		}
		return options;
	});

	const selectCount = $derived.by(() => {
		return options.filter((o) => o.checked || o.selected).length;
	});

	// Filter options based on search query (skip if externalFilter is true)
	const filteredOptions = $derived.by(() => {
		if (externalFilter || !searchQuery.trim()) return sortedOptions;

		const query = searchQuery.toLowerCase();
		const result: MenuOption<T>[] = [];
		let currentGroupLabel: MenuOption<T> | null = null;

		for (const option of sortedOptions) {
			if (option.type === 'group-label') {
				currentGroupLabel = option;
			} else if (option.label.toLowerCase().includes(query)) {
				// Add group label if we haven't already
				if (currentGroupLabel && !result.includes(currentGroupLabel)) {
					result.push(currentGroupLabel);
				}
				result.push(option);
			}
		}

		return result;
	});

	const totalOptions = $derived(filteredOptions.length);
	const hasOptions = $derived(totalOptions > 0);
	const showLoading = $derived(loading);
	const showEmptyState = $derived(
		!showLoading && !hasOptions && !searchQuery.trim() && emptyMessage
	);
	const showNoResults = $derived(
		!showLoading && !hasOptions && searchQuery.trim() && noResultsMessage
	);

	const isSkippable = (opt: MenuOption<T> | undefined) =>
		!opt || opt.disabled || opt.type === 'group-label' || opt.type === 'divider';

	const findFirstValidIndex = (): number => {
		for (let i = 0; i < filteredOptions.length; i++) {
			if (!isSkippable(filteredOptions[i])) return i;
		}
		return -1;
	};

	// Whenever the search query changes the visible options, reset the active
	// index to the first available option so the selection starts at the top.
	$effect(() => {
		if (!searchEnabled) return;
		// Touch filteredOptions so the effect reruns when the filter result changes
		void filteredOptions;
		const first = findFirstValidIndex();
		activeIndex = first;
	});

	// Visual highlight: when input is focused (activeIndex === -1), highlight the first option
	const displayHighlightIndex = $derived(
		activeIndex === -1 && searchEnabled ? findFirstValidIndex() : activeIndex
	);

	const findIndex = (from: number, step: 1 | -1): number => {
		let i = from;
		while (i >= 0 && i < totalOptions && isSkippable(filteredOptions[i])) i += step;
		return i >= 0 && i < totalOptions ? i : -1;
	};

	const activateOption = (index: number) => {
		activeIndex = index;
		containerRef?.focus();
	};

	const focusSearchInput = () => {
		activeIndex = -1;
		const inputElement = searchInputWrapperRef?.querySelector('input') as HTMLInputElement;
		inputElement?.focus();
	};

	const focusFirstOption = () => {
		const i = findIndex(0, 1);
		if (i >= 0) activateOption(i);
	};

	const navigateTo = (direction: 'next' | 'prev' | 'first' | 'last') => {
		if (direction === 'first') return searchEnabled ? focusSearchInput() : focusFirstOption();
		if (direction === 'last') {
			const i = findIndex(totalOptions - 1, -1);
			if (i >= 0) activateOption(i);
			return;
		}

		// When input is focused, use displayHighlightIndex as the starting point
		// so arrow down moves from the visually highlighted first option to the second
		const currentIndex = activeIndex === -1 ? displayHighlightIndex : activeIndex;
		const step = direction === 'next' ? 1 : -1;
		const i = findIndex(currentIndex + step, step);

		if (i >= 0) activateOption(i);
		else if (direction === 'prev' && searchEnabled) focusSearchInput();
	};

	const navigate = async (direction: 'next' | 'prev' | 'first' | 'last') => {
		navigateTo(direction);
		await tick();
		if (activeIndex >= 0) {
			containerRef
				?.querySelector(`[data-menu-index="${activeIndex}"]`)
				?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
		}
	};

	// Handle option click
	const handleOptionClick = async (
		e: MouseEvent | KeyboardEvent,
		option: MenuOption<T>,
		index: number
	) => {
		// Ignore mouse-triggered interactions until the user has actively moved
		// the mouse after the menu was opened. Keyboard interactions should
		// always work.
		if (e instanceof MouseEvent && !mouseInteractionEnabled) {
			return;
		}

		e.stopPropagation();
		e.preventDefault();

		if (!option.action || pendingOptionId !== null) return;

		activeIndex = index;
		pendingOptionId = String(option.id);
		try {
			await option.action(option);
		} finally {
			pendingOptionId = null;
		}
	};

	onMount(async () => {
		await tick();

		if (searchEnabled) {
			focusSearchInput();
		} else if (containerRef) {
			focusFirstOption();
		}

		// Disable mouse interactions until the user moves the mouse. This avoids
		// accidental hover/clicks when the cursor is hidden while navigating the
		// menu with the keyboard.
		mouseInteractionEnabled = false;
		mouseMoveHandler = () => {
			mouseInteractionEnabled = true;
			if (mouseMoveHandler) {
				window.removeEventListener('mousemove', mouseMoveHandler);
				mouseMoveHandler = null;
			}
		};
		window.addEventListener('mousemove', mouseMoveHandler);

		setContext('menuFocus', true);

		registerCommand('menu.selectNext', () => navigate('next'));
		registerCommand('menu.selectPrevious', () => navigate('prev'));
		registerCommand('menu.confirm', async () => {
			if (pendingOptionId !== null) return;
			let option = activeIndex >= 0 ? filteredOptions[activeIndex] : undefined;
			// If no option selected (e.g., focus is on search input), select the first available option
			if (!option && totalOptions > 0) {
				const firstIndex = findIndex(0, 1);
				if (firstIndex >= 0) option = filteredOptions[firstIndex];
			}
			if (option?.checkbox) option.onCheckClick?.(option);
			else if (option?.action) {
				const index = option ? filteredOptions.indexOf(option) : -1;
				if (index >= 0) {
					pendingOptionId = String(option.id);
					try {
						await option.action(option);
					} finally {
						pendingOptionId = null;
					}
				}
			}
		});
		registerCommand('menu.close', () => {
			if (onClose) {
				onClose();
			} else if (getContext().modalOpen) {
				modalStore.set(null);
			}
		});
	});

	onDestroy(() => {
		if (mouseMoveHandler) {
			window.removeEventListener('mousemove', mouseMoveHandler);
			mouseMoveHandler = null;
		}
		setContext('menuFocus', false);
		unregisterCommand('menu.selectNext');
		unregisterCommand('menu.selectPrevious');
		unregisterCommand('menu.confirm');
		unregisterCommand('menu.close');
	});
</script>

<div
	class="menu-wrapper"
	class:noBorder
	style="{style}; {maxHeight ? `max-height: ${maxHeight}px;` : ''} {minHeight
		? `min-height: ${minHeight}px;`
		: ''} {width
		? `min-width: ${typeof width === 'number' ? `${width}px` : width}; max-width: ${typeof width === 'number' ? `${width}px` : width};`
		: ''}"
>
	{#if searchEnabled}
		<div
			bind:this={searchInputWrapperRef}
			onfocusin={() => (activeIndex = -1)}
			style="border-bottom: var(--border); z-index: 1;"
		>
			<Input
				bind:value={searchQuery}
				placeholder={searchPlaceholder}
				validator={searchValidator}
				style="width: 100%;"
				size="lg"
				autofocus
				noRadius
				noBorder
			/>
		</div>
	{/if}

	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		bind:this={containerRef}
		class="menu-options scrollable"
		role="menu"
		aria-label="Menu options"
		tabindex={0}
		onfocus={() => {
			// When container receives focus, activate first option only if no option is already active
			if (activeIndex >= 0 || totalOptions <= 0) return;
			focusFirstOption();
		}}
	>
		{#if showLoading}
			<div class="menu-state">
				<Loader />
			</div>
		{:else if showEmptyState}
			<div class="menu-state">
				<p>{emptyMessage}</p>
			</div>
		{:else if showNoResults}
			<div class="menu-state">
				<p>{noResultsMessage}</p>
			</div>
		{:else}
			{#each filteredOptions as option, i (`${String(option.id)}-${i}`)}
				{@const isActive = i === displayHighlightIndex}
				{@const isPending = String(option.id) === pendingOptionId}
				{#if option.type === 'group-label'}
					<div class="menu-group-label">
						<p>{option.label}</p>
					</div>
				{:else if option.type === 'divider'}
					<div class="menu-divider"></div>
				{:else}
					<!-- svelte-ignore a11y_interactive_supports_focus -->
					<div
						data-menu-index={i}
						role="menuitem"
						onmouseenter={() => {
							if (mouseInteractionEnabled) activeIndex = i;
						}}
						class="menu-option-row"
						data-active={highlightActiveOption && isActive}
						data-selected={option.checked || option.selected}
						aria-busy={isPending}
					>
						{#if option.checkbox || selectCount > 0}
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<div
								class="menu-option-left-slot"
								onclick={(e) => {
									if (e instanceof MouseEvent && !mouseInteractionEnabled) return;
									if (!option.checkbox) handleOptionClick(e, option, i);
									if (option.checkbox) option.onCheckClick?.(option);
								}}
							>
								{#if option.checkbox}
									<Checkbox
										checked={option.checked ?? false}
										onchange={() => option.onCheckClick?.(option)}
										size="sm"
									/>
								{:else if option.selected}
									<Icon icon="check" size={16} />
								{/if}
							</div>
						{/if}
						{#if optionContent && option.customData !== undefined}
							<div
								class="menu-option-custom"
								role="button"
								tabindex="0"
								onclick={(e) => handleOptionClick(e, option, i)}
								onkeydown={(e) => e.key === 'Enter' && handleOptionClick(e, option, i)}
							>
								{@render optionContent(option.customData)}
							</div>
						{:else}
							<Button
								id="menu-option-{option.id}"
								size="md"
								content={option.label}
								leftIcon={option.icon}
								iconSize={option.iconSize || 16}
								disabled={option.disabled || isPending}
								loading={isPending}
								badge={option.badge}
								onclick={(e) => handleOptionClick(e, option, i)}
								ariaLabel={option.label}
								truncate={false}
								noBounce
							/>
						{/if}
					</div>
				{/if}
			{/each}
		{/if}
	</div>
</div>

<style>
	.menu-wrapper {
		display: flex;
		flex-direction: column;
		min-width: 200px;
		background-color: var(--gray-0);
		border: var(--border);
		border-radius: var(--br-sm);
		box-shadow: var(--gray-400) 0px 7px 29px 0px;
		overflow: hidden;
	}

	.menu-wrapper.noBorder {
		border: none;
	}

	.menu-options {
		overflow-y: auto;
	}

	.menu-state {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-lg);
		color: var(--gray-700);
	}

	.menu-group-label {
		padding: var(--space-sm) var(--space-sm-md);
		border-bottom: var(--border);
	}

	.menu-group-label p {
		font-size: var(--fs-xs);
		color: var(--gray-800);
		font-weight: 400;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.menu-option-row {
		display: flex;
		align-items: center;
		background: var(--gray-0);
		height: 36px;
		transition:
			background-color 0.1s ease-out,
			color 0.1s ease-out;
	}
	.menu-option-row[data-active='true'] {
		background: var(--gray-550);
	}

	:global(.menu-option-row p),
	:global(.menu-option-row span) {
		color: var(--gray-800);
	}
	:global(.menu-option-row[data-active='true'] p),
	:global(.menu-option-row[data-active='true'] span),
	:global(.menu-option-row[data-selected='true'] p),
	:global(.menu-option-row[data-selected='true'] span) {
		color: var(--gray-900) !important;
	}

	:global(.menu-option-row .button) {
		background-color: transparent !important;
		width: 100%;
		justify-content: flex-start;
		gap: var(--space-xs);
	}

	.menu-divider {
		border-top: var(--border);
	}

	.menu-option-left-slot {
		display: flex;
		align-items: center;
		justify-content: end;
		flex-shrink: 0;
		width: 24px;
		min-height: 32px;
	}

	.menu-option-custom {
		flex: 1;
		min-width: 0;
		display: flex;
		align-items: center;
		padding: var(--space-xs-sm) var(--space-sm);
		width: 100%;
		box-sizing: border-box;
	}
</style>
