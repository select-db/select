<script lang="ts">
	import { onMount } from 'svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import Select from '$lib/system/Select/Select.svelte';
	import Checkbox from '$lib/system/Checkbox/Checkbox.svelte';

	type VarType = 'text' | 'integer' | 'decimal' | 'boolean' | 'date' | 'timestamp' | 'time';

	const typeOptions: { value: VarType; label: string }[] = [
		{ value: 'text', label: 'Text' },
		{ value: 'integer', label: 'Integer' },
		{ value: 'decimal', label: 'Decimal' },
		{ value: 'boolean', label: 'Boolean' },
		{ value: 'date', label: 'Date' },
		{ value: 'timestamp', label: 'Timestamp' },
		{ value: 'time', label: 'Time' }
	];

	type Props = {
		vars: string[];
		initial: Record<string, string>;
		initialTypes: Record<string, string>;
		onSubmit: (vals: Record<string, string>, types: Record<string, string>) => void;
		onCancel: () => void;
		onValuesChange?: (vals: Record<string, string>, types: Record<string, string>) => void;
	};

	let { vars, initial, initialTypes, onSubmit, onCancel, onValuesChange }: Props = $props();

	let formEl = $state<HTMLFormElement | undefined>();

	let values = $state<Record<string, string>>(
		Object.fromEntries(vars.map((name) => [name, initial[name] ?? '']))
	);
	let types = $state<Record<string, VarType>>(
		Object.fromEntries(vars.map((name) => [name, (initialTypes[name] as VarType) ?? 'text']))
	);

	// Notify parent on every change so values can be persisted between modal open/close.
	$effect(() => {
		const valSnap = { ...values };
		const typeSnap = { ...types };
		onValuesChange?.(valSnap, typeSnap);
	});

	onMount(() => {
		const first = formEl?.querySelector<HTMLElement>('input, select, button[role="combobox"]');
		first?.focus();
	});

	function boolValue(name: string): boolean {
		return values[name] === 'true';
	}
	function setBool(name: string, checked: boolean) {
		values[name] = checked ? 'true' : 'false';
	}

	const allFilled = $derived(
		vars.every((name) => {
			if (types[name] === 'boolean') return true;
			return (values[name] ?? '').trim() !== '';
		})
	);

	function submit() {
		if (!allFilled) return;
		onSubmit({ ...values }, { ...types });
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && allFilled) {
			e.preventDefault();
			submit();
		}
		if (e.key === 'Escape') {
			onCancel();
		}
	}

	function inputType(t: VarType): string {
		switch (t) {
			case 'integer':
			case 'decimal':
				return 'number';
			case 'date':
				return 'date';
			case 'timestamp':
				return 'datetime-local';
			case 'time':
				return 'time';
			default:
				return 'text';
		}
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<form class="modal" bind:this={formEl} onkeydown={handleKeydown}>
	<ModalHeader title="Set variables" />

	<div class="body">
		{#each vars as name (name)}
			<div class="row">
				<label class="varname" for="var-{name}">${name}</label>
				<div class="controls">
					<Select
						options={typeOptions}
						value={types[name]}
						onchange={(v) => {
							const t = v as VarType;
							types[name] = t;
							if (t === 'boolean' && values[name] !== 'true' && values[name] !== 'false') {
								values[name] = 'false';
							}
						}}
						width={120}
						size="xs"
					/>
					{#if types[name] === 'boolean'}
						<Checkbox
							checked={boolValue(name)}
							onchange={(checked) => setBool(name, checked)}
							label={boolValue(name) ? 'true' : 'false'}
							size="sm"
						/>
					{:else}
						<input
							id="var-{name}"
							class="value-input"
							type={inputType(types[name])}
							value={values[name]}
							placeholder="..."
							oninput={(e) => (values[name] = e.currentTarget.value)}
							step={types[name] === 'decimal' ? 'any' : undefined}
							autocomplete="off"
							spellcheck={false}
						/>
					{/if}
				</div>
			</div>
		{/each}
	</div>

	<div class="footer">
		<Button content="Cancel" emphasis="low" onclick={onCancel} />
		<Button content="Run" emphasis="high" disabled={!allFilled} onclick={submit} />
	</div>
</form>

<style>
	.modal {
		display: flex;
		flex-direction: column;
		background-color: var(--gray-0);
		border-radius: var(--br-sm);
		overflow: hidden;
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
		padding: var(--space-md);
		background-color: var(--gray-0);
		max-height: 60vh;
		overflow-y: auto;
	}

	.row {
		display: flex;
		align-items: center;
		gap: var(--space-sm);
	}

	.varname {
		font-size: var(--fs-sm);
		font-weight: var(--fw-md);
		color: var(--gray-1000);
		font-family: var(--font-mono);
		min-width: 120px;
		flex-shrink: 0;
	}

	.controls {
		display: flex;
		gap: var(--space-sm);
		align-items: center;
		flex: 1;
		min-width: 0;

		height: 28px;
		align-items: stretch;
	}

	.value-input {
		flex: 1;
		background-color: var(--gray-0);
		border: var(--border);
		border-radius: var(--br-xs);
		padding: var(--space-sm);
		font-size: var(--fs-sm);
		color: var(--gray-1000);
		min-width: 0;
	}

	.value-input:hover,
	.value-input:focus {
		background-color: var(--gray-100);
		outline: none;
	}

	.value-input:focus {
		border-color: var(--gray-700);
	}

	.footer {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-xs);
		padding: var(--space-sm);
		border-top: var(--border);
		background-color: var(--gray-0);
	}
</style>
