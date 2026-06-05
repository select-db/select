import { writable } from 'svelte/store';

export type TooltipData = {
	id: string;
	text: string;
	anchor: HTMLElement;
	position: 'top' | 'bottom' | 'left' | 'right';
	actionable: boolean;
	appearDelay: number;
	capitalize?: boolean;
};

// Track when the last tooltip was hidden for quick-switch behavior
export let lastHideTime = 0;
export function setLastHideTime(time: number) {
	lastHideTime = time;
}

function createTooltipStore() {
	const { subscribe, set } = writable<TooltipData | null>(null);

	return {
		subscribe,
		show: (data: TooltipData) => set(data),
		hide: () => set(null)
	};
}

export const tooltipStore = createTooltipStore();
