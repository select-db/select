<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import Input from '$lib/system/Input/Input.svelte';
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

	// Seeded once: the form owns these from here on, and re-deriving them from
	// the props would discard what the user has typed.
	let values = $state<Record<string, string>>(
		untrack(() => Object.fromEntries(vars.map((name) => [name, initial[name] ?? ''])))
	);
	let types = $state<Record<string, VarType>>(
		untrack(() =>
			Object.fromEntries(vars.map((name) => [name, (initialTypes[name] as VarType) ?? 'text']))
		)
	);

	// Notify parent on every change so values can be persisted between modal open/close.
	$effect(() => {
		const valSnap = Object.fromEntries(
			Object.entries(values).map(([k, v]) => [k, String(v ?? '')])
		);
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

	// String(), not a bare .trim(): an input of type number binds a number back,
	// or null once it is cleared, and neither has .trim(). Calling it on one
	// threw inside this derivation, which left Run disabled for good -- so an
	// integer, decimal, date, timestamp or time variable could be typed but
	// never run.
	const allFilled = $derived(
		vars.every((name) => {
			if (types[name] === 'boolean') return true;
			return String(values[name] ?? '').trim() !== '';
		})
	);

	function submit() {
		if (!allFilled) return;
		// And back to strings on the way out, for the same reason: everything
		// downstream of here, from the tab that stores them to the Go side that
		// substitutes them, is typed as Record<string, string>.
		onSubmit(
			Object.fromEntries(Object.entries(values).map(([k, v]) => [k, String(v ?? '')])),
			{ ...types }
		);
	}

	async function handleRun() {
		submit();
	}

	async function handleCancel() {
		onCancel();
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

	function inputType(t: VarType): 'text' | 'number' | 'date' | 'time' | 'datetime-local' {
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
<form bind:this={formEl} onkeydown={handleKeydown}>
	<ModalHeader title="Set variables" />

	<ModalBody style="display: flex; flex-direction: column; gap: var(--space-sm); max-height: 60vh;">
		{#each vars as name (name)}
			<div class="row">
				<span class="varname">${name}</span>
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
						<Input
							type={inputType(types[name])}
							bind:value={values[name]}
							placeholder="..."
							step={types[name] === 'decimal' ? 'any' : undefined}
							style="flex: 1; min-width: 0;"
						/>
					{/if}
				</div>
			</div>
		{/each}
	</ModalBody>

	<ModalFooter
		mainAction={{ label: 'Run', action: handleRun, disabled: !allFilled }}
		secondaryAction={{ label: 'Cancel', action: handleCancel }}
	/>
</form>

<style>
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
</style>
