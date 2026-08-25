import type { ContextMenuOption } from '$lib/system/ContextMenu/types';
import { modalStore } from '$lib/system/Modal/ModalStore';
import { loadSchema } from '$lib/utils/query/loadSchema';
import { must, tryCatch } from '$lib/utils/tryCatch';
import type { graph } from '$lib/wailsjs/go/models';
import DatabaseSystemInfo from '$lib/components/views/FileSystem/modals/ItemInfoModal.svelte';

import * as fs from '$lib/wailsjs/go/fs_provider/FSProvider';
import { navigateToDatabase } from '$lib/components/views/shared/navigateToDatabase';
import { navigateToSchema } from '$lib/components/views/Schema/navigateToSchema';
import { fileSystemOptions } from './fileOptions';
import { createFileInFolder, createFolderInFolder } from './rootOptions';

export const databaseOptions = [
	{
		label: 'Refresh',
		action: (onClose, database: graph.DBInstanceNode) => {
			void loadSchema({ database, noCache: true, silent: true });
			onClose();
		}
	},
	{
		label: 'Edit...',
		runOnDoubleClick: true,
		action: async (onClose, database: graph.DBInstanceNode) => {
			await navigateToDatabase(database);
			onClose();
		}
	},
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
		action: async (onClose, { uri }: graph.DBInstanceNode) => {
			await must(
				tryCatch(fs.Delete, {
					uri,
					recursive: true
				})
			);
			onClose();
		}
	}
] satisfies ContextMenuOption[];
