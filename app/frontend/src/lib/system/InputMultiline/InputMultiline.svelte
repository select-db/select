<script lang="ts">
	import { onMount } from 'svelte';

	type InputMultilineProps = {
		value?: string;
		focused?: boolean;
		active?: boolean;
		placeholder?: string;
		autofocus?: boolean;
		onkeydown?: (e: KeyboardEvent) => void;
		onsubmit?: () => void;

		style?: string;
		size?: 'md' | 'lg';
		noRadius?: boolean;
		noBorder?: boolean;
	};

	let {
		value = $bindable(),
		focused = $bindable(false),
		active = $bindable(false),
		placeholder = '',
		autofocus = false,
		onkeydown,
		onsubmit,

		style,
		size = 'md',
		noRadius = false,
		noBorder = false
	}: InputMultilineProps = $props();

	let el: HTMLDivElement;

	export const focus = () => el?.focus();

	const isEmpty = $derived(!value || value.trim().length === 0);

	function syncFromDom() {
		const text = el?.innerText ?? '';
		if (text === value) return;
		value = text;
	}

	function syncToDom() {
		if (!el) return;
		const target = value ?? '';
		if (el.innerText === target) return;
		// eslint-disable-next-line svelte/no-dom-manipulating
		el.innerText = target;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && e.shiftKey) {
			// Shift+Enter: insert newline
			document.execCommand('insertLineBreak');
			e.preventDefault();
			return;
		}

		if (e.key === 'Enter') {
			// Enter: submit
			e.preventDefault();
			onsubmit?.();
			return;
		}

		onkeydown?.(e);
	}

	function handlePaste(e: ClipboardEvent) {
		e.preventDefault();
		const text = e.clipboardData?.getData('text/plain') ?? '';
		document.execCommand('insertText', false, text);
	}

	$effect(() => {
		if (!el) return;
		syncToDom();
	});

	onMount(() => {
		if (!autofocus) return;
		requestAnimationFrame(() => {
			el?.focus();
		});
	});
</script>

<div
	bind:this={el}
	contenteditable="true"
	role="textbox"
	aria-multiline="true"
	tabindex="0"
	aria-label={placeholder}
	aria-placeholder={placeholder}
	data-placeholder={placeholder}
	data-empty={isEmpty}
	{style}
	class={size}
	class:noRadius
	class:noBorder
	onfocus={() => (focused = true)}
	onblur={() => {
		focused = false;
		active = false;
		syncFromDom();
	}}
	onmousedown={() => (active = true)}
	onmouseup={() => (active = false)}
	onmouseleave={() => (active = false)}
	oninput={syncFromDom}
	onkeydown={handleKeydown}
	onpaste={handlePaste}
></div>

<style>
	div[contenteditable='true'] {
		border: var(--border);
		transition: all 0.2s;

		/* Multiline */
		white-space: pre-wrap;
		word-break: break-word;
		overflow-wrap: break-word;
		min-height: 1.5em;
		max-height: 12em;
		overflow-y: auto;
		outline: none;

		font-size: var(--fs-sm);
		font-weight: var(--fw-light);
	}

	div[contenteditable='true'][data-empty='true']::before {
		content: attr(data-placeholder);
		color: var(--gray-800);
	}

	div[contenteditable='true']:not(.noRadius) {
		border-radius: var(--br-sm);
	}

	div[contenteditable='true'].noBorder {
		border: none;
	}

	div[contenteditable='true']:hover,
	div[contenteditable='true']:focus {
		background-color: var(--gray-100);
	}
	div[contenteditable='true']:focus:not(.noBorder) {
		border-color: var(--gray-700);
	}

	div[contenteditable='true'].md {
		padding: var(--space-sm);
	}
	div[contenteditable='true'].lg {
		padding: var(--space-sm-md);
	}
</style>
