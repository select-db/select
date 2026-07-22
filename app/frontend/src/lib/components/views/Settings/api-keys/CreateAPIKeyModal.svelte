<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Button from '$lib/system/Button/Button.svelte';
	import Menu from '$lib/system/Menu/Menu.svelte';
	import Portal from '$lib/system/Portal/Portal.svelte';
	import FloatingBox from '$lib/system/FloatingBox/FloatingBox.svelte';
	import { tryCatch } from '$lib/utils/tryCatch';
	import type { apikey } from '$lib/wailsjs/go/models';

	type Role = { id: string; name: string };
	type Props = {
		onClose: () => void;
		roles: Role[];
		onCreate: (params: apikey.CreateParams) => Promise<void>;
	};

	let { onClose, roles, onCreate }: Props = $props();

	let name = $state('');
	let selectedRoleIds = new SvelteSet<string>();
	let creating = $state(false);

	const expiryPresets = [
		{ id: 'never', label: 'Never', days: 0 },
		{ id: '30', label: '30 days', days: 30 },
		{ id: '90', label: '90 days', days: 90 },
		{ id: '365', label: '1 year', days: 365 }
	];
	let expiryId = $state('never');
	let expiryAnchor = $state<HTMLElement | null>(null);
	let roleAnchor = $state<HTMLElement | null>(null);

	let expiryLabel = $derived(expiryPresets.find((p) => p.id === expiryId)?.label ?? 'Never');
	let selectedRoleNames = $derived(
		roles.filter((r) => selectedRoleIds.has(r.id)).map((r) => r.name)
	);

	function toggleRole(id: string) {
		if (selectedRoleIds.has(id)) selectedRoleIds.delete(id);
		else selectedRoleIds.add(id);
	}

	let roleOptions = $derived(
		roles.map((r) => ({
			id: r.id,
			label: r.name,
			checkbox: true,
			checked: selectedRoleIds.has(r.id),
			action: (o: { id: string }) => toggleRole(o.id),
			onCheckClick: (o: { id: string }) => toggleRole(o.id)
		}))
	);

	let expiryOptions = $derived(
		expiryPresets.map((p) => ({
			id: p.id,
			label: p.label,
			selected: p.id === expiryId,
			action: () => {
				expiryId = p.id;
				expiryAnchor = null;
			}
		}))
	);

	function expiresAtISO(): string | undefined {
		const preset = expiryPresets.find((p) => p.id === expiryId);
		if (!preset || preset.days === 0) return undefined;
		return new Date(Date.now() + preset.days * 24 * 60 * 60 * 1000).toISOString();
	}

	let hint = $derived.by(() => {
		if (!name.trim()) return 'Enter a name for the API key';
		if (selectedRoleIds.size === 0) return 'Select at least one role';
		return null;
	});

	const handleCreate = async () => {
		const trimmed = name.trim();
		if (!trimmed || selectedRoleIds.size === 0) return;
		creating = true;
		await tryCatch(onCreate, {
			name: trimmed,
			role_ids: [...selectedRoleIds],
			expires_at: expiresAtISO()
		} as apikey.CreateParams);
		creating = false;
	};
</script>

<ModalHeader title="New API key" />
<ModalBody>
	<div class="field">
		<span class="label">Name</span>
		<Input bind:value={name} placeholder="e.g. claude-readonly" autofocus />
	</div>

	<div class="field">
		<span class="label">Roles</span>
		<Button
			content={selectedRoleNames.length ? selectedRoleNames.join(', ') : 'Select roles'}
			emphasis="low"
			size="sm"
			truncate
			onclick={(e) => {
				roleAnchor = e.currentTarget as HTMLElement;
			}}
		/>
	</div>

	<div class="field">
		<span class="label">Expires</span>
		<Button
			content={expiryLabel}
			emphasis="low"
			size="sm"
			onclick={(e) => {
				expiryAnchor = e.currentTarget as HTMLElement;
			}}
		/>
	</div>
	{#if hint}
		<p class="hint">{hint}</p>
	{/if}
</ModalBody>
<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onClose(), disabled: creating }}
	mainAction={{
		label: 'Create',
		action: handleCreate,
		disabled: creating || !name.trim() || selectedRoleIds.size === 0
	}}
/>

{#if roleAnchor}
	<Portal>
		<FloatingBox anchor={roleAnchor} backdrop onBackdropClick={() => (roleAnchor = null)}>
			<Menu
				options={roleOptions}
				searchEnabled={true}
				searchPlaceholder="Search roles…"
				emptyMessage="No roles"
				noResultsMessage="No matching roles"
				width={240}
				onClose={() => (roleAnchor = null)}
			/>
		</FloatingBox>
	</Portal>
{/if}

{#if expiryAnchor}
	<Portal>
		<FloatingBox anchor={expiryAnchor} backdrop onBackdropClick={() => (expiryAnchor = null)}>
			<Menu options={expiryOptions} width={180} onClose={() => (expiryAnchor = null)} />
		</FloatingBox>
	</Portal>
{/if}

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-xs);
		margin-bottom: var(--space-sm-md);
	}
	.label {
		font-size: var(--fs-xs);
		font-weight: var(--fw-md);
		color: var(--gray-900);
	}
	.hint {
		font-size: var(--fs-xs);
		color: var(--gray-800);
		text-align: right;
	}
</style>
