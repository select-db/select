import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { loadSchema } from '$lib/utils/query/loadSchema';
import type * as graph from '$lib/wails/graph';
import DatasourceSystemInfo from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';

import { deleteEntries } from '$lib/components/views/shared/deleteEntries';
import { navigateToDatasource } from '$lib/components/views/shared/navigateToDatasource';
import { navigateToSchema } from '$lib/components/views/Schema/navigateToSchema';
import { fileSystemOptions } from './fileOptions';
import { renameOption } from './helpers';
import { createFileInFolder, createFolderInFolder } from './rootOptions';

export const datasourceOptions = [
	{
		label: 'Refresh',
		action: (onClose, datasource: graph.DatasourceNode) => {
			void loadSchema({ datasource });
			onClose();
		}
	},
	{
		label: 'Edit...',
		action: async (onClose, datasource: graph.DatasourceNode) => {
			await navigateToDatasource(datasource);
			onClose();
		}
	},
	renameOption,
	{
		label: 'Infos...',
		action: async (onClose, datasource: graph.DatasourceNode) => {
			modalStore.set({
				content: () => DatasourceSystemInfo,
				props: { item: datasource },
				width: 600
			});
			onClose();
		}
	},
	{
		label: 'Schema...',
		action: (onClose, datasource: graph.DatasourceNode) => {
			void navigateToSchema(datasource.id);
			onClose();
		}
	},
	{
		label: '',
		divider: true
	},
	{
		label: 'New folder...',
		action: async (onClose, datasource: graph.DatasourceNode) => {
			await createFolderInFolder(datasource);
			onClose();
		}
	},
	{
		label: 'New file...',
		action: async (onClose, datasource: graph.DatasourceNode) => {
			await createFileInFolder(datasource);
			onClose();
		}
	},
	...fileSystemOptions,
	{
		label: '',
		divider: true
	},
	{
		label: 'Delete',
		action: async (onClose, datasource: graph.DatasourceNode) => {
			await deleteEntries([datasource]);
			onClose();
		}
	}
] satisfies ContextMenuOption[];
