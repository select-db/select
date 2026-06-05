<script lang="ts">
	type CheckboxProps = {
		checked?: boolean;
		indeterminate?: boolean;
		denied?: boolean;
		disabled?: boolean;
		label?: string;
		size?: 'sm' | 'md';
		onchange?: (checked: boolean) => void;
	};

	let {
		checked = $bindable(false),
		indeterminate = $bindable(false),
		denied = false,
		disabled = false,
		label,
		size = 'md',
		onchange
	}: CheckboxProps = $props();

	const handleChange = (e: Event) => {
		e.stopPropagation();
		e.preventDefault();
		const target = e.target as HTMLInputElement;
		checked = target.checked;
		indeterminate = false;
		onchange?.(checked);
	};
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<label
	class="checkbox-wrapper truncate"
	class:disabled
	class:sm={size === 'sm'}
	class:md={size === 'md'}
	class:denied
	onclick={(e) => e.stopPropagation()}
>
	<input
		type="checkbox"
		bind:checked
		bind:indeterminate
		{disabled}
		onchange={handleChange}
		class="checkbox-input"
		aria-label={label}
	/>
	<span class="checkbox-box" aria-hidden="true">
		<span class="checkbox-tick"></span>
	</span>
	{#if label}
		<span class="checkbox-label truncate">{label}</span>
	{/if}
</label>

<style>
	.checkbox-wrapper {
		display: inline-flex;
		align-items: center;
		gap: var(--space-sm);
		user-select: none;
		contain: layout style paint;
		outline: none;
	}

	.checkbox-wrapper.disabled {
		cursor: not-allowed;
		opacity: 0.5;
	}

	.checkbox-input {
		position: absolute;
		opacity: 0;
		width: 0;
		height: 0;
		margin: 0;
		padding: 0;
		pointer-events: none;
	}

	.checkbox-box {
		position: relative;
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 16px;
		height: 16px;
		border: var(--border);
		border-color: var(--gray-700);
		background-color: var(--gray-100);
		border-radius: var(--br-xs);
		transition:
			background-color 0.2s ease,
			border-color 0.2s ease,
			box-shadow 0.2s ease;
		contain: layout style paint;
	}

	.checkbox-wrapper.sm .checkbox-box {
		width: 12px;
		height: 12px;
	}

	.checkbox-tick {
		position: absolute;
		width: 100%;
		height: 100%;
		opacity: 0;
		transform: scale(0);
		transition:
			opacity 0.15s ease 0.05s,
			transform 0.15s cubic-bezier(0.34, 1.56, 0.64, 1);
		pointer-events: none;
		contain: strict;
	}

	.checkbox-tick::before {
		content: '';
		position: absolute;
		left: 50%;
		top: 50%;
		width: 4px;
		height: 8px;
		border: solid var(--gray-1000);
		border-width: 0 2px 2px 0;
		transform: translate(-50%, -60%) rotate(45deg);
	}

	.checkbox-wrapper.sm .checkbox-tick::before {
		width: 2.5px;
		height: 5px;
		border-width: 0 1.25px 1.25px 0;
	}

	.checkbox-input:checked + .checkbox-box {
		background-color: var(--blue);
		border-color: var(--blue);
		box-shadow: inset 0 1px 2px var(--shadow);
	}

	.checkbox-input:checked + .checkbox-box .checkbox-tick {
		opacity: 1;
		transform: scale(1);
	}

	:global([data-theme='light']) .checkbox-input:checked + .checkbox-box .checkbox-tick::before {
		border-color: white;
	}

	.checkbox-input:indeterminate + .checkbox-box {
		background-color: var(--gray-300);
		box-shadow: inset 0 1px 2px var(--shadow);
	}

	.checkbox-input:indeterminate + .checkbox-box .checkbox-tick {
		opacity: 1;
		transform: scale(1);
	}

	.checkbox-input:indeterminate + .checkbox-box .checkbox-tick::before {
		width: 8px;
		height: 0;
		border-color: var(--gray-700);
		border-width: 0 0 2px 0;
		transform: translate(-50%, -50%);
		border-radius: 1px;
	}

	.checkbox-wrapper.sm .checkbox-input:indeterminate + .checkbox-box .checkbox-tick::before {
		width: 5px;
		border-width: 0 0 1.25px 0;
	}

	.checkbox-wrapper:not(.disabled):hover .checkbox-input:indeterminate + .checkbox-box {
		background-color: var(--gray-400);
	}

	.checkbox-input:focus-visible + .checkbox-box {
		outline: 0.5px solid var(--blue-glow);
		outline-offset: 0.5px;
	}

	.checkbox-wrapper:not(.disabled):hover .checkbox-box {
		border-color: var(--blue);
		background-color: var(--gray-100);
	}

	.checkbox-wrapper:not(.disabled):hover .checkbox-input:checked + .checkbox-box {
		background-color: var(--blue-glow);
		box-shadow: inset 0 1px 2px var(--shadow);
	}

	.checkbox-label {
		color: var(--gray-800);
		font-size: var(--fs-sm);
		line-height: 1.4;
		transition: color 0.15s ease;
	}

	.checkbox-wrapper:not(.disabled):hover .checkbox-label,
	.checkbox-input:checked ~ .checkbox-label {
		color: var(--gray-1000);
	}

	.checkbox-wrapper.disabled .checkbox-label {
		color: var(--gray-700);
	}

	/* Denied state: red box with X */
	.checkbox-wrapper.denied .checkbox-box {
		background-color: var(--red);
		border-color: var(--red);
		box-shadow: inset 0 1px 2px var(--shadow);
	}

	.checkbox-wrapper.denied .checkbox-tick {
		opacity: 1;
		transform: scale(1);
	}

	.checkbox-wrapper.denied .checkbox-tick::before,
	.checkbox-wrapper.denied .checkbox-tick::after {
		content: '';
		position: absolute;
		left: 50%;
		top: 50%;
		width: 65%;
		height: 2px;
		background: white;
		border: none;
		border-radius: 1px;
		transform-origin: center;
	}

	.checkbox-wrapper.denied .checkbox-tick::before {
		transform: translate(-50%, -50%) rotate(45deg);
	}

	.checkbox-wrapper.denied .checkbox-tick::after {
		transform: translate(-50%, -50%) rotate(-45deg);
	}

	.checkbox-wrapper.sm.denied .checkbox-tick::before,
	.checkbox-wrapper.sm.denied .checkbox-tick::after {
		width: 60%;
		height: 1.5px;
	}

	.checkbox-wrapper:not(.disabled).denied:hover .checkbox-box {
		background-color: color-mix(in srgb, var(--red) 80%, white);
		border-color: color-mix(in srgb, var(--red) 80%, white);
	}
</style>
