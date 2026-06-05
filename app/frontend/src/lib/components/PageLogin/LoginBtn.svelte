<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { modalStore } from '$lib/system/Modal/ModalStore';

	import GithubDeviceCode from './modal/GithubDeviceCode.svelte';

	const openModal = () => {
		modalStore.set({
			content: () => GithubDeviceCode,
			props: {},
			width: 300
		});
	};
</script>

<Button
	onclick={openModal}
	content="Log in with Github"
	leftIcon="github"
	iconSize={16}
	emphasis="high"
/>
