import { untrack } from 'svelte';
import type { Tab } from '$lib/components/Layout/layoutStore';
import { updateTab } from '$lib/components/Layout/layoutStore';
import type { CreateChatReturn } from '$lib/components/views/Chat/core/tanstack-svelte/types';

export function useChatPersistence(
	chat: CreateChatReturn<any>,
	getTab: () => Tab,
	sessionId: string,
	getInputText: () => string,
	getSelectedValue: () => string,
	getScrollTop: () => number
) {
	$effect(() => {
		getInputText();
		getSelectedValue();
		getScrollTop();
		chat.messages;
		const t = untrack(getTab);
		const serialized = chat.messages
			.filter((m) => m.role !== 'system')
			.map((m) => ({
				id: m.id ?? crypto.randomUUID(),
				role: m.role,
				parts: m.parts ?? []
			}));
		updateTab({
			...t,
			chat: {
				...t.chat!,
				task: undefined, // tasks are one-time triggers; never re-persist
				sessionId,
				inputValue: getInputText(),
				selectedModel: getSelectedValue(),
				messages: serialized,
				scrollTop: getScrollTop()
			}
		});
	});
}
