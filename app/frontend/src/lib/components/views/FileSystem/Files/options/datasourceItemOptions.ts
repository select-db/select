import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import ItemInfoModal from '../../modals/ItemInfoModal.svelte';
import type * as graph from '$lib/wails/graph';
import {
	isPreviewableDatasourceItem,
	viewTableData
} from '$lib/components/views/shared/viewTableData';

const infoOption = {
	label: 'Infos...',
	runOnClick: true,
	action: async (onClose, item: graph.DatasourceItemNode) => {
		modalStore.set({
			content: () => ItemInfoModal,
			props: { item },
			width: 600
		});
		onClose();
	}
} satisfies ContextMenuOption;

const viewDataOption = {
	label: 'View data',
	runOnDoubleClick: true,
	action: async (onClose, item: graph.DatasourceItemNode) => {
		onClose();
		await viewTableData(item);
	}
} satisfies ContextMenuOption;

export const getDatasourceItemOptions = (item: graph.DatasourceItemNode): ContextMenuOption[] =>
	isPreviewableDatasourceItem(item) ? [viewDataOption, infoOption] : [infoOption];
