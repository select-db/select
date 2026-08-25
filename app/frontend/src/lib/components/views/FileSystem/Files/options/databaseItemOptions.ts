import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import DatabaseSystemInfo from '../../modals/ItemInfoModal.svelte';
import type * as graph from '$lib/wails/graph';
import { isPreviewableDbItem, viewTableData } from '$lib/components/views/shared/viewTableData';

const infoOption = {
	label: 'Infos...',
	action: async (onClose, item: graph.DBInstanceItemNode) => {
		modalStore.set({
			content: () => DatabaseSystemInfo,
			props: { item },
			width: 600
		});
		onClose();
	}
} satisfies ContextMenuOption;

const viewDataOption = {
	label: 'View data',
	runOnDoubleClick: true,
	action: async (onClose, item: graph.DBInstanceItemNode) => {
		onClose();
		await viewTableData(item);
	}
} satisfies ContextMenuOption;

export const getDatabaseItemOptions = (item: graph.DBInstanceItemNode): ContextMenuOption[] =>
	isPreviewableDbItem(item) ? [viewDataOption, infoOption] : [infoOption];
