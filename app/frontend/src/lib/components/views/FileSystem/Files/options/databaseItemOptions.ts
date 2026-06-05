import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import DatabaseSystemInfo from '../../modals/ItemInfoModal.svelte';
import type { graph } from '$lib/wailsjs/go/models';

export const databaseItemOptions = [
	{
		label: 'Infos...',
		action: async (onClose, item: graph.DBInstanceItemNode) => {
			modalStore.set({
				content: () => DatabaseSystemInfo,
				props: { item },
				width: 600
			});
			onClose();
		}
	}
] satisfies ContextMenuOption[];
