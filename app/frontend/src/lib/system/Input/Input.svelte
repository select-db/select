<script lang="ts">
	import { onMount } from 'svelte';
	import { setContext } from '$lib/stores/keybindingsContextStore';
	import Icon from '$lib/system/Icon/Icon.svelte';

	type InputProps = {
		value?: string | number;
		placeholder?: string;
		autofocus?: boolean;
		clearable?: boolean;
		onkeydown?: (e: KeyboardEvent) => void;
		oninput?: (e: Event) => void;
		onclear?: () => void;

		type?: 'text' | 'number' | 'password' | 'date' | 'time' | 'datetime-local';
		multiline?: boolean;
		rows?: number;
		min?: number;
		max?: number;
		step?: number | 'any';
		style?: string;
		size?: 'md' | 'lg';
		emphasis?: 'high' | 'low';
		noRadius?: boolean;
		noBorder?: boolean;
		/** Returns an error message if invalid, or null/undefined if valid. */
		validator?: (value: string) => string | null | undefined;
	};

	let {
		value = $bindable(),
		placeholder,
		autofocus,
		clearable = false,
		onkeydown,
		oninput,
		onclear,

		type = 'text',
		multiline = false,
		rows = 3,
		min,
		max,
		step,
		style,
		size = 'md',
		emphasis = 'high',
		noRadius = false,
		noBorder = false,
		validator
	}: InputProps = $props();

	let validationError = $derived.by(() => {
		if (!validator) return null;
		return validator(String(value ?? '')) ?? null;
	});

	let inputRef = $state<HTMLInputElement>();
	let showPassword = $state(false);

	const effectiveType = $derived(type === 'password' && showPassword ? 'text' : type);
	const needsWrap = $derived(type === 'password' || clearable || multiline);

	export const focus = () => inputRef?.focus();

	const handleFocus = () => setContext('inputFocus', true);
	const handleBlur = () => {
		setContext('inputFocus', false);
	};

	function clear() {
		value = '';
		onclear?.();
		inputRef?.focus();
	}

	onMount(() => {
		if (!autofocus || !inputRef) return;
		inputRef.focus();
		if (typeof inputRef.select === 'function') inputRef.select();
	});
</script>

{#snippet inputEl()}
	<!-- The caller opts in, and the desktop app's modals and pickers rely on it
	     to be usable from the keyboard alone. -->
	<!-- svelte-ignore a11y_autofocus -->
	<input
		bind:this={inputRef}
		bind:value
		{placeholder}
		{autofocus}
		{onkeydown}
		{oninput}
		{style}
		type={effectiveType}
		{min}
		{max}
		{step}
		class={size}
		class:noRadius
		class:noBorder
		class:low={emphasis === 'low'}
		class:masked={type === 'password' && !showPassword}
		class:has-error={!!validationError}
		autocapitalize="off"
		autocomplete="off"
		spellcheck="false"
		onfocus={handleFocus}
		onblur={handleBlur}
	/>
{/snippet}

{#snippet textareaEl()}
	<!-- svelte-ignore a11y_autofocus -->
	<textarea
		bind:value
		{placeholder}
		{autofocus}
		{rows}
		{style}
		class={`scrollable ${size}`}
		class:noRadius
		class:noBorder
		class:low={emphasis === 'low'}
		class:masked={type === 'password' && !showPassword}
		class:has-error={!!validationError}
		autocapitalize="off"
		autocomplete="off"
		spellcheck="false"
		onfocus={handleFocus}
		onblur={handleBlur}></textarea>
{/snippet}

{#if needsWrap}
	<div class="input-wrap" class:has-value={!!value} class:password-wrap={type === 'password'}>
		{#if multiline}
			{@render textareaEl()}
		{:else}
			{@render inputEl()}
		{/if}

		{#if type === 'password'}
			<button
				class="toggle-password-btn"
				class:multiline
				type="button"
				aria-label={showPassword ? 'Hide password' : 'Show password'}
				tabindex="-1"
				onmousedown={(e) => {
					e.preventDefault();
					showPassword = !showPassword;
				}}
			>
				<Icon icon={showPassword ? 'eye-off' : 'eye'} size={14} />
			</button>
		{:else if clearable}
			<button
				class="clear-btn"
				aria-label="Clear"
				tabindex="-1"
				onmousedown={(e) => {
					e.preventDefault();
					clear();
				}}
			>
				<svg width="10" height="10" viewBox="0 0 10 10" fill="none">
					<path
						d="M1 1L9 9M9 1L1 9"
						stroke="currentColor"
						stroke-width="1.5"
						stroke-linecap="round"
					/>
				</svg>
			</button>
		{/if}
	</div>
{:else}
	{@render inputEl()}
{/if}

{#if validationError}
	<p class="error-msg">{validationError}</p>
{/if}

<style>
	input,
	textarea {
		background-color: var(--gray-200);
		border: var(--border);
		transition: all 0.2s;
		box-shadow: var(--shadow-subtle);
		color: var(--gray-800);
	}

	input::placeholder,
	textarea::placeholder {
		color: var(--gray-800);
	}

	input:not(.noRadius),
	textarea:not(.noRadius) {
		border-radius: var(--br-sm);
	}

	input.noBorder,
	textarea.noBorder {
		border: none;
	}

	input:hover,
	input:focus,
	textarea:hover,
	textarea:focus {
		background-color: var(--gray-300);
		color: var(--gray-1000);
	}
	input:focus:not(.noBorder),
	textarea:focus:not(.noBorder) {
		border-color: var(--gray-600);
	}

	input.low,
	textarea.low {
		background-color: transparent;
		border: 0.5px solid transparent;
	}
	input.low::placeholder,
	textarea.low::placeholder {
		color: var(--gray-700);
	}
	input.low:hover,
	input.low:focus,
	textarea.low:hover,
	textarea.low:focus {
		background-color: var(--gray-100);
		border-color: var(--border-color);
	}

	input.md,
	textarea.md {
		padding: var(--space-sm);
	}
	input.lg,
	textarea.lg {
		padding: var(--space-sm-md);
	}

	textarea {
		resize: vertical;
		width: 100%;
	}

	input.masked,
	textarea.masked {
		-webkit-text-security: disc;
		color: var(--gray-800);
	}

	input.has-error,
	textarea.has-error {
		border-color: var(--red);
	}

	input.has-error:focus,
	textarea.has-error:focus {
		border-color: var(--red);
	}

	.error-msg {
		position: absolute;
		bottom: -0.8em;
		left: var(--space-xs-sm);
		color: var(--red);
		background-color: var(--gray-0);
		padding: var(--space-xxs) var(--space-xs) var(--space-xxs) var(--space-xs);
		border-radius: var(--br-sm);
		font-size: var(--fs-xxs);
		margin-top: var(--space-xxs);
	}

	/* Wrapper for clearable / password inputs */
	.input-wrap {
		width: 100%;
		position: relative;
		display: flex;
		align-items: center;
	}

	.input-wrap input {
		width: 100%;
		padding-right: 28px;
	}

	.password-wrap input {
		padding-right: 32px;
	}

	/* Clear button */
	.clear-btn {
		position: absolute;
		right: var(--space-xs-sm);
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		padding: 0;
		background: none;
		border: none;
		border-radius: var(--br-sm);
		color: var(--gray-700);
		opacity: 0;
		transition:
			opacity 0.1s,
			color 0.1s;
		pointer-events: none;
	}

	.input-wrap.has-value:hover .clear-btn,
	.input-wrap.has-value:has(:focus) .clear-btn {
		opacity: 1;
		pointer-events: auto;
	}

	.clear-btn:hover {
		color: var(--gray-1000);
		background-color: var(--gray-200);
	}

	/* Password toggle */
	.toggle-password-btn {
		position: absolute;
		right: var(--space-xs-sm);
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-xs);
		background: none;
		border: none;
		border-radius: var(--br-sm);
		color: var(--gray-700);
	}

	.toggle-password-btn:hover {
		color: var(--gray-1000);
	}

	.toggle-password-btn.multiline {
		position: absolute;
		top: var(--space-xs-sm);
		right: var(--space-xs-sm);
	}

	textarea.masked {
		-webkit-text-security: disc;
		color: var(--gray-800);
	}

	.input-container {
		display: flex;
		flex-direction: column;
		width: 100%;
	}

	.has-error :global(input),
	.has-error :global(textarea) {
		background-color: var(--red-100, rgba(255, 0, 0, 0.05));
	}

	.has-error :global(input:focus),
	.has-error :global(textarea:focus) {
		background-color: var(--red-100, rgba(255, 0, 0, 0.05));
	}

	.has-error :global(input:focus::placeholder),
	.has-error :global(textarea:focus::placeholder) {
		color: var(--gray-1000);
	}
</style>
