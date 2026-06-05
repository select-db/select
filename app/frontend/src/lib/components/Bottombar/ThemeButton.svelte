<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import { onMount, onDestroy } from 'svelte';

	let theme: 'dark' | 'light' | 'system' = 'system';

	function getSystemTheme(): 'dark' | 'light' {
		return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
	}

	function applyTheme() {
		const resolved = theme === 'system' ? getSystemTheme() : theme;
		document.documentElement.setAttribute('data-theme', resolved);
		localStorage.setItem('theme', theme);
	}

	let systemQuery: MediaQueryList | undefined;
	function listenSystemTheme() {
		systemQuery = window.matchMedia('(prefers-color-scheme: dark)');
		systemQuery.addEventListener('change', onSystemThemeChange);
	}
	function onSystemThemeChange() {
		if (theme === 'system') applyTheme();
	}

	onMount(() => {
		const saved = localStorage.getItem('theme');
		if (saved) theme = saved as 'dark' | 'light' | 'system';
		applyTheme();
		listenSystemTheme();
	});

	onDestroy(() => {
		systemQuery?.removeEventListener('change', onSystemThemeChange);
	});

	function toggleTheme() {
		theme = theme === 'dark' ? 'light' : 'dark';
		applyTheme();
	}
</script>

<Button
	iconSize={19}
	onclick={toggleTheme}
	leftIcon={theme === 'dark' ? 'light-mode' : 'dark-mode'}
	noRadius
/>
