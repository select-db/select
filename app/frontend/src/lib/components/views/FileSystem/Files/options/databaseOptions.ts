import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { loadSchema } from '$lib/utils/query/loadSchema';
import type * as graph from '$lib/wails/graph';
import DatabaseSystemInfo from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';

import { deleteEntries } from '$lib/components/views/shared/deleteEntries';
import { navigateToDatabase } from '$lib/components/views/shared/navigateToDatabase';
import { navigateToSchema } from '$lib/components/views/Schema/navigateToSchema';
import { fileSystemOptions } from './fileOptions';
import { renameOption } from './helpers';
import { createFileInFolder, createFolderInFolder } from './rootOptions';

export const databaseOptions = [
	{
		label: 'Refresh',
		action: (onClose, database: graph.DBInstanceNode) => {
			void loadSchema({ database, noCache: true });
			onClose();
		}
	},
	{
		label: 'Edit...',
		action: async (onClose, database: graph.DBInstanceNode) => {
			await navigateToDatabase(database);
			onClose();
		}
	},
	renameOption,
	{
		label: 'Infos...',
		action: async (onClose, database: graph.DBInstanceNode) => {
			modalStore.set({
				content: () => DatabaseSystemInfo,
				props: { item: database },
				width: 600
			});
			onClose();
		}
	},
	{
		label: 'Schema...',
		action: (onClose, database: graph.DBInstanceNode) => {
			void navigateToSchema(database.id);
			onClose();
		}
	},
	{
		label: '',
		divider: true
	},
	{
		label: 'New folder...',
		action: async (onClose, database: graph.DBInstanceNode) => {
			await createFolderInFolder(database);
			onClose();
		}
	},
	{
		label: 'New file...',
		action: async (onClose, database: graph.DBInstanceNode) => {
			await createFileInFolder(database);
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
		action: async (onClose, database: graph.DBInstanceNode) => {
			await deleteEntries([database]);
			onClose();
		}
	}
] satisfies ContextMenuOption[];
