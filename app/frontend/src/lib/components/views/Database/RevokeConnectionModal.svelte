<script lang="ts">
	import ModalHeader from '$lib/system/Modal/ModalHeader.svelte';
	import ModalBody from '$lib/system/Modal/ModalBody.svelte';
	import ModalFooter from '$lib/system/Modal/ModalFooter.svelte';

	type Props = {
		/** The shared connections about to be revoked. Never empty. */
		names: string[];
		onConfirm: () => void;
		onCancel: () => void;
	};

	let { names, onConfirm, onCancel }: Props = $props();

	const one = $derived(names.length === 1);
</script>

<ModalHeader
	icon="db"
	title={one ? `Delete ${names[0]}?` : `Delete ${names.length} shared connections?`}
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
		{one ? `${names[0]} is a shared connection` : 'These are shared connections'}. The credentials
		live on the server rather than in this workspace, so deleting the files here would leave them
		reachable by everyone else who has access.
	</p>
	<p class="text">
		Deleting drops those credentials for the whole workspace. Anyone querying stops being able to,
		and the connection has to be set up again from scratch to come back.
	</p>
</ModalBody>

<ModalFooter
	secondaryAction={{ label: 'Cancel', action: async () => onCancel() }}
	mainAction={{ label: 'Delete and revoke', action: async () => onConfirm() }}
/>

<style>
	.text {
		margin: 0 0 var(--space-sm);
		color: var(--gray-1000);
	}

	.names {
		margin: 0 0 var(--space-sm);
		padding-left: var(--space-md);
		color: var(--gray-1000);
	}
</style>
