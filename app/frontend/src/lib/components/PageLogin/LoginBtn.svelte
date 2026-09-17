<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { modalStore } from '$lib/system/Modal/ModalStore';

	import { startLogin } from './loginFlowStore';
	import { loginProviders, type LoginProvider } from './loginProviders';

	import DeviceCodeAuth from './modal/DeviceCodeAuth.svelte';

	function openModal(provider: LoginProvider) {
		// Not awaited: it resolves when the provider authorizes, which is the
		// whole point of the modal being able to close in the meantime.
		void startLogin(provider);
		modalStore.set({
			content: () => DeviceCodeAuth,
			props: {},
			width: 300
		});
	}
</script>

{#each loginProviders as provider (provider.id)}
	<Button
		onclick={() => openModal(provider)}
		content="Log in with {provider.name}"
		leftIcon={provider.icon}
		iconSize={16}
		emphasis="high"
		noLoader
	/>
{/each}
