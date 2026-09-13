<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';

	type Props = {
		/** The shared connections about to be revoked. Never empty. */
		names: string[];
		/** What the person actually asked for. Revoking is the consequence. */
		verb: 'Delete' | 'Revoke';
		onConfirm: () => void;
		onCancel: () => void;
	};

	let { names, verb, onConfirm, onCancel }: Props = $props();

	const one = $derived(names.length === 1);
</script>

<ModalHeader
	icon="db"
	title={one ? `${verb} ${names[0]}?` : `${verb} ${names.length} shared connections?`}
/>

<ModalBody>
	{#if !one}
		<ul class="names">
			{#each names as name (name)}
				<li>{name}</li>
			{/each}
		</ul>
	{/if}

	<p class="text">
		This drops those credentials for the whole workspace. Anyone querying stops being able to, and
		the connection has to be set up again from scratch to come back.
	</p>
</ModalBody>

<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onCancel() }}
	mainAction={{
		label: verb === 'Delete' ? 'Delete and revoke' : 'Revoke',
		action: async () => onConfirm(),
		emphasis: 'warning'
	}}
/>

<style>
	.text {
		margin: 0 0 var(--space-sm);
		color: var(--gray-1000);
		white-space: collapse;
	}

	.names {
		margin: 0 0 var(--space-sm);
		padding-left: var(--space-md);
		color: var(--gray-1000);
	}
</style>
