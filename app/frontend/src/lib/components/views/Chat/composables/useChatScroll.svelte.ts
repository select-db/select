import { tick } from 'svelte';
import { debounce } from '$lib/utils/debounce';
import type { Tab } from '$lib/components/Layout/layoutStore';

const SCROLL_AT_BOTTOM_THRESHOLD = 30;

type UseChatScrollOptions = {
	getScrollEl: () => HTMLDivElement | undefined;
	getIsStreaming: () => boolean;
	getMessages: () => unknown[];
};

export function useChatScroll(tab: Tab, options: UseChatScrollOptions) {
	const { getScrollEl, getIsStreaming, getMessages } = options;

	let scrollTop = $state(tab.chat?.scrollTop ?? 0);
	let atBottom = $state(true);

	function updateAtBottom(el: HTMLDivElement) {
		atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - SCROLL_AT_BOTTOM_THRESHOLD;
	}

	const saveScroll = debounce(() => {
		const el = getScrollEl();
		if (el) scrollTop = el.scrollTop;
	}, 150);

	function onScroll(e: Event) {
		const el = e.currentTarget as HTMLDivElement;
		updateAtBottom(el);
		saveScroll();
	}

	async function restore() {
		await tick();
		requestAnimationFrame(() => {
			const el = getScrollEl();
			if (!el) return;
			const saved = tab.chat?.scrollTop;
			el.scrollTop = saved ?? el.scrollHeight;
			updateAtBottom(el);
		});
	}

	$effect(() => {
		if (!getIsStreaming() || !atBottom) return;
		getMessages();
		const el = getScrollEl();
		if (!el) return;
		tick().then(() => {
			const current = getScrollEl();
			if (current) current.scrollTop = current.scrollHeight;
		});
	});

	async function scrollToBottom() {
		atBottom = true;
		await tick();
		requestAnimationFrame(() => {
			const el = getScrollEl();
			if (!el) return;
			el.scrollTop = el.scrollHeight;
		});
	}

	return { get scrollTop() { return scrollTop; }, onScroll, restore, scrollToBottom };
}
