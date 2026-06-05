<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		normalizeKey,
		findMatchingKeybinding,
		initKeybindings
	} from '$lib/stores/keybindingsStore';
	import { getContext } from '$lib/stores/keybindingsContextStore';
	import { executeCommand } from '$lib/stores/commandRegistry';

	let initialized = false;
	let cmdHeldInMenu = false;

	function handleKeydown(e: KeyboardEvent) {
		if (!initialized) return;

		const key = normalizeKey(e);
		if (!key || key === 'shift' || key === 'alt' || key === 'ctrl' || key === 'cmd') {
			return;
		}

		const context = getContext();

		// Track if cmd+p is used for menu navigation (VS Code-style select on release)
		if (key === 'cmd+p' && context.menuFocus) cmdHeldInMenu = true;

		// Escape closes menu regardless of modifiers (e.g., cmd+escape while navigating with cmd+p)
		if (e.key === 'Escape' && context.menuFocus) {
			e.preventDefault();
			e.stopPropagation();
			executeCommand('menu.close');
			cmdHeldInMenu = false;
			return;
		}

		const keybinding = findMatchingKeybinding(key, context);

		if (keybinding) {
			e.preventDefault();
			e.stopPropagation();
			executeCommand(keybinding.command);
		}
	}

	function handleKeyup(e: KeyboardEvent) {
		if (!initialized) return;

		// When cmd is released while navigating menu with cmd+p, confirm selection
		if ((e.key === 'Meta' || e.key === 'Control') && cmdHeldInMenu) {
			const context = getContext();
			if (context.menuFocus) executeCommand('menu.confirm');
			cmdHeldInMenu = false;
		}
	}

	onMount(async () => {
		await initKeybindings();
		initialized = true;

		window.addEventListener('keydown', handleKeydown, true);
		window.addEventListener('keyup', handleKeyup, true);
	});

	onDestroy(() => {
		window.removeEventListener('keydown', handleKeydown, true);
		window.removeEventListener('keyup', handleKeyup, true);
	});
</script>
