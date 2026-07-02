<script lang="ts">
	import type { git } from '$lib/wailsjs/go/models';
	import { GetBranches, SwitchBranch } from '$lib/wailsjs/go/git/Git';

	import { Menu } from '$lib/system/Menu';
	import type { MenuOption } from '$lib/system/Menu';
	import { modalStore } from '$lib/system/Modal/ModalStore';
	import { loadGitStatus } from '$lib/components/views/Git/gitStore';
	import { notifyError } from '$lib/system/Notifications/notificationsStore';

	import { must, tryCatch } from '$lib/utils/tryCatch';

	let branches = $state<git.BranchInfo[]>([]);
	let loading = $state(true);
	let searchQuery = $state('');

	const loadBranches = async () => {
		loading = true;
		const [result, err] = await tryCatch(GetBranches);
		if (err) notifyError(err.message);
		loading = false;
		branches = result || [];
	};

	const switchToBranch = async (branchName: string) => {
		await must(tryCatch(SwitchBranch, { branchName }));
		await loadGitStatus();
		modalStore.set(null);
	};

	const createAndCheckoutBranch = async () => {
		const trimmedQuery = searchQuery.trim();
		if (!trimmedQuery) return;
		await switchToBranch(trimmedQuery);
	};

	// Load branches when component mounts
	$effect(() => {
		loadBranches();
	});

	// Transform branches into flat menu options with group labels
	const menuOptions = $derived.by((): MenuOption[] => {
		const options: MenuOption[] = [];

		for (const branch of branches) {
			options.push({
				id: branch.name,
				label: branch.name,
				icon: 'github-branch',
				badge: branch.isCurrent ? 'current' : undefined,
				action: async () => {
					if (branch.isCurrent) return;
					await switchToBranch(branch.name);
				}
			});
		}

		const trimmedQuery = searchQuery.trim();
		const exactMatch =
			trimmedQuery && branches.some((b) => b.name.toLowerCase() === trimmedQuery.toLowerCase());
		const showCreateOption = trimmedQuery && !exactMatch;

		// Add create option if applicable
		if (showCreateOption) {
			options.push({
				id: '__create__',
				label: `Create and checkout '${trimmedQuery}'`,
				icon: 'plus',
				action: createAndCheckoutBranch
			});
		}

		return options;
	});
</script>

<Menu
	{loading}
	bind:searchQuery
	options={menuOptions}
	searchEnabled
	searchPlaceholder="Search or create branch..."
	emptyMessage="No branches found"
	noResultsMessage={`No branches match "${searchQuery}"`}
	onClose={() => modalStore.set(null)}
	maxHeight={400}
	width={400}
	noBorder
/>
