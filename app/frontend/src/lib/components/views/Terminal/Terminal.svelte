<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { Terminal as XTerm } from '@xterm/xterm';
	import { FitAddon } from '@xterm/addon-fit';
	import { WebLinksAddon } from '@xterm/addon-web-links';
	import '@xterm/xterm/css/xterm.css';

	import { EventsOn } from '$lib/wails/events';
	import * as TerminalBackend from '$lib/bindings/selectDb/internal/terminal/terminal';
	import { type Tab, updateTab } from '$lib/components/Layout/layoutStore';
	import { getTerminalTheme } from './terminalTheme';
	import Header from './Header.svelte';

	type Props = {
		tab: Tab;
	};

	let { tab }: Props = $props();
	const sessionId = tab.terminal!.sessionId;
	let currentShell = $state(tab.terminal!.shell);

	let containerEl: HTMLDivElement;
	let xterm: XTerm | null = null;
	let fitAddon: FitAddon | null = null;
	let unsubOutput: (() => void) | null = null;
	let unsubExit: (() => void) | null = null;
	let resizeObserver: ResizeObserver | null = null;
	let destroyed = false;

	const subscribe = () => {
		unsubOutput?.();
		unsubExit?.();

		unsubOutput = EventsOn(`terminal:output:${sessionId}`, (encoded: string) => {
			const chunks = tab.terminal!.outputChunks!;
			chunks.push(encoded);
			if (chunks.length > 2000) {
				tab.terminal!.outputChunks = chunks.slice(-1000);
			}
			const bytes = Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
			xterm?.write(bytes);
		});

		unsubExit = EventsOn(`terminal:exit:${sessionId}`, () => {
			tab.terminal!.exited = true;
			xterm?.writeln('\r\n[Process exited]');
		});
	};

	const handleShellChange = async (shell: string) => {
		unsubOutput?.();
		unsubExit?.();
		await TerminalBackend.Destroy(sessionId).catch(() => {});
		if (destroyed) return;

		tab.terminal!.outputChunks = [];
		tab.terminal!.exited = false;
		currentShell = shell;
		updateTab({ ...tab, terminal: { ...tab.terminal!, shell } });

		xterm?.reset();

		try {
			await TerminalBackend.Create(sessionId, shell);
		} catch (err) {
			if (!destroyed) xterm?.writeln(`\r\nFailed to create terminal session: ${err}`);
			return;
		}
		if (destroyed) return;

		if (xterm) {
			await TerminalBackend.Resize(sessionId, xterm.cols, xterm.rows).catch(() => {});
		}
		if (destroyed) return;

		subscribe();
		xterm?.focus();
	};

	onMount(async () => {
		if (!tab.terminal!.outputChunks) {
			tab.terminal!.outputChunks = [];
		}

		xterm = new XTerm({
			cursorBlink: true,
			fontSize: 12,
			lineHeight: 1.3,
			fontFamily: "'SF Mono', 'Menlo', 'Monaco', 'Consolas', monospace",
			fontWeight: 200,
			theme: getTerminalTheme(),
			allowProposedApi: true
		});

		fitAddon = new FitAddon();
		xterm.loadAddon(fitAddon);
		xterm.loadAddon(new WebLinksAddon());

		xterm.open(containerEl);
		fitAddon.fit();

		// Replay buffered output from previous mount
		for (const encoded of tab.terminal!.outputChunks) {
			const bytes = Uint8Array.from(atob(encoded), (c) => c.charCodeAt(0));
			xterm.write(bytes);
		}
		if (tab.terminal!.exited) {
			xterm.writeln('\r\n[Process exited]');
		}

		// Create session (silently skip if already exists from previous mount)
		try {
			await TerminalBackend.Create(sessionId, currentShell);
		} catch (err) {
			if (destroyed) return;
			if (!String(err).includes('already exists')) {
				xterm.writeln(`\r\nFailed to create terminal session: ${err}`);
				return;
			}
		}
		if (destroyed) return;

		await TerminalBackend.Resize(sessionId, xterm.cols, xterm.rows).catch(() => {});
		if (destroyed) return;

		subscribe();

		xterm.onData((data: string) => {
			const encoded = btoa(data);
			TerminalBackend.Write(sessionId, encoded).catch(() => {});
		});

		xterm.onBinary((data: string) => {
			const encoded = btoa(data);
			TerminalBackend.Write(sessionId, encoded).catch(() => {});
		});

		resizeObserver = new ResizeObserver(() => {
			if (!fitAddon || !xterm) return;
			fitAddon.fit();
			TerminalBackend.Resize(sessionId, xterm.cols, xterm.rows).catch(() => {});
		});
		resizeObserver.observe(containerEl);

		xterm.focus();
	});

	onDestroy(() => {
		destroyed = true;
		unsubOutput?.();
		unsubExit?.();
		resizeObserver?.disconnect();
		xterm?.dispose();
		xterm = null;
	});
</script>

<div class="terminal-wrapper">
	<Header shell={currentShell} onShellChange={handleShellChange} />
	<div class="terminal-container" bind:this={containerEl}></div>
</div>

<style>
	.terminal-wrapper {
		position: relative;
		display: flex;
		flex-direction: column;
		width: 100%;
		height: 100%;
		overflow: hidden;
	}

	.terminal-container {
		flex: 1;
		min-height: 0;
		overflow: hidden;
		background-color: var(--gray-200);
	}

	.terminal-container :global(.xterm) {
		height: 100%;
		padding: var(--space-sm) 0 0 var(--space-sm);
		font-weight: var(--terminal-font-weight, 200);
		text-rendering: optimizeLegibility;
	}

	.terminal-container :global(.xterm .xterm-scrollable-element) {
		background-color: var(--gray-200) !important;
	}

	.terminal-container :global(.xterm-viewport) {
		background-color: var(--gray-200) !important;
	}
</style>
