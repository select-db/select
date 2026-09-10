<script lang="ts">
	import Button from '$lib/system/Button/Button.svelte';
	import Input from '$lib/system/Input/Input.svelte';
	import Loader from '$lib/system/Loader/Loader.svelte';
	import { scrollShadow } from '$lib/actions/scrollShadow';
	import { AlertType } from '$lib/system/Alert/types';
	import { modalStore } from '$lib/system/Modal/ModalStore';

	import { notify } from '$lib/system/Notifications/notificationsStore';

	import { must, tryCatch } from '$lib/utils/tryCatch';

	import { workspaceGraphStore } from '$lib/utils/graph/workspaceGraphStore';
	import {
		gitWorkspaceStatusStore,
		gitFileStatusStore,
		loadGitStatus
	} from '$lib/components/views/Git/gitStore';
	import { mapGitFilesToNodes } from '$lib/components/views/Git/helpers';

	import FileItems from '$lib/components/views/FileSystem/Files/FileItems.svelte';
	import { expandItem, expandedItemIdsStore } from '$lib/components/views/shared/sharedStore';
	import {
		buildVisibilityIndex,
		updateScrollWindow
	} from '$lib/components/views/FileSystem/Files/helpers/visibilityStore';
	import { throttle } from '$lib/utils/throttle';
	import PullOptionsModal from '$lib/components/views/Git/PullOptionsModal.svelte';
	import ForcePushModal from '$lib/components/views/Git/ForcePushModal.svelte';

	import {
		PushWorkspaceRepo,
		PushForceWithLease,
		CommitChanges,
		PullWorkspaceRepo,
		PullWithRebase,
		ResetBranchToRemote
	} from '$lib/bindings/selectDb/internal/git/git';

	import * as graph from '$lib/wails/graph';
	import type { Component } from 'svelte';

	function isPushRejected(err: Error): boolean {
		const msg = err?.message?.toLowerCase() ?? '';
		return (
			msg.includes('rejected') || msg.includes('non-fast-forward') || msg.includes('! [rejected]')
		);
	}

	// Use the reactive stores for both git status types
	const workspaceGitStatus = $derived($gitWorkspaceStatusStore);
	const detailedStatus = $derived($gitFileStatusStore);

	let commitMessage = $state('');

	const pushToRemote = async () => {
		const [, err] = await tryCatch(PushWorkspaceRepo);
		if (err && isPushRejected(err)) {
			modalStore.set({
				content: (() => ForcePushModal) as () => Component,
				props: {
					branchName: detailedStatus?.branch,
					onConfirm: async () => {
						await must(tryCatch(PushForceWithLease));
						notify({ type: AlertType.Success, message: 'Pushed to remote (force with lease)' });
						await loadGitStatus();
					}
				},
				width: 420
			});
			return;
		}
		if (err) {
			must([null, err]);
			return;
		}
		notify({ type: AlertType.Success, message: 'Pushed to remote' });
		await loadGitStatus();
	};

	const pullFromRemote = async () => {
		const commitsAhead = detailedStatus?.commitsAhead ?? 0;
		const commitsBehind = detailedStatus?.commitsBehind ?? 0;
		if (commitsAhead > 0 && commitsBehind > 0) {
			modalStore.set({
				content: (() => PullOptionsModal) as () => Component,
				props: {
					onChoice: async (choice: 'merge' | 'rebase' | 'reset') => {
						if (choice === 'merge') {
							await must(tryCatch(PullWorkspaceRepo));
						} else if (choice === 'rebase') {
							await must(tryCatch(PullWithRebase));
						} else {
							await must(tryCatch(ResetBranchToRemote));
						}
						notify({
							type: AlertType.Success,
							message:
								choice === 'merge'
									? 'Pulled from remote'
									: choice === 'rebase'
										? 'Pulled with rebase'
										: 'Reset to remote'
						});
						await loadGitStatus();
					}
				},
				width: 380
			});
			return;
		}
		await must(tryCatch(PullWorkspaceRepo));
		notify({ type: AlertType.Success, message: 'Pulled from remote' });
		await loadGitStatus();
	};

	// Create placeholder folders for git changes
	const createGitFolder = (name: string, id: string, files: graph.FileNode[]): graph.FolderNode => {
		return graph.newFolderNode({
			id,
			uri: id,
			type: 'folder',
			name,
			folder_id: '',
			files,
			folders: [],
			db_instances: [],
			badges: [String(files.length)]
		});
	};

	const workspaceId = $derived($workspaceGraphStore?.id);

	const stagedFolder = $derived.by(() => {
		if (!detailedStatus || !workspaceId) return [];
		const files = mapGitFilesToNodes(detailedStatus.staged, workspaceId);
		return [createGitFolder('Staged changes', 'git::staged', files)];
	});

	const unstagedFolder = $derived.by(() => {
		if (!detailedStatus || !workspaceId) return [];
		const files = [
			...mapGitFilesToNodes(detailedStatus.unstaged, workspaceId),
			...mapGitFilesToNodes(detailedStatus.untracked, workspaceId)
		];
		return [createGitFolder('Unstaged changes', 'git::unstaged', files)];
	});

	// Ensure git folders are expanded by default
	$effect(() => {
		expandItem('git::staged');
		expandItem('git::unstaged');
	});

	// Virtual scrolling
	let scrollContainer: HTMLDivElement = $state()!;

	// Rebuild visibility index when git folders or expanded state changes
	$effect(() => {
		const folders = [...stagedFolder, ...unstagedFolder];
		const expandedIds = $expandedItemIdsStore;
		buildVisibilityIndex('git', folders, [], [], [], expandedIds);
		// Update scroll window immediately after building index
		if (scrollContainer) {
			updateScrollWindow('git', scrollContainer.scrollTop, scrollContainer.clientHeight);
		}
	});

	const handleScroll = throttle(() => {
		if (scrollContainer) {
			updateScrollWindow('git', scrollContainer.scrollTop, scrollContainer.clientHeight);
		}
	}, 16);

	const commit = async () => {
		await must(tryCatch(CommitChanges, { message: commitMessage }));
		notify({ type: AlertType.Success, message: 'Commit created' });
		commitMessage = '';
		await loadGitStatus();
	};
</script>

<div class="github-panel" data-test="git.panel">
	{#if !workspaceGitStatus}
		<div class="section space x y">
			<Loader />
		</div>
	{:else if !workspaceGitStatus?.gitAvailable}
		<div class="section space x y">
			<p class="section-title">Git not available</p>
			<p class="hint">
				Git is not installed or not available in your PATH. <br />
				Install Git to enable version control for this folder.
			</p>
		</div>
	{:else if workspaceGitStatus.isGitRepo}
		<!-- Push and pull need a remote; committing does not. -->
		{#if detailedStatus}
			{@const commitsAhead = detailedStatus?.commitsAhead ?? 0}
			{@const commitsBehind = detailedStatus?.commitsBehind ?? 0}
			{@const stagedCount = detailedStatus.staged.length}

			<div class="section space x" style="padding-top: var(--space-sm-md)">
				<div class="field">
					<Input bind:value={commitMessage} placeholder="Commit message" />
				</div>
				<div class="actions">
					<Button
						content="Commit"
						size="sm"
						onclick={commit}
						badge={stagedCount}
						emphasis={stagedCount > 0 ? 'high' : 'low'}
						iconSize={14}
					/>
					{#if workspaceGitStatus.hasRemote}
						<Button
							content="Push"
							size="sm"
							onclick={pushToRemote}
							badge={commitsAhead}
							emphasis={commitsAhead > 0 ? 'high' : 'low'}
							iconSize={14}
						/>
						<Button
							content="Pull"
							size="sm"
							onclick={pullFromRemote}
							badge={commitsBehind}
							emphasis={commitsBehind > 0 ? 'high' : 'low'}
							iconSize={14}
						/>
					{/if}
				</div>
			</div>

			<div
				class="section no-scrollbar overflow-x-only"
				style="border-top: var(--border); flex-grow: 1; min-width: 0"
				bind:this={scrollContainer}
				use:scrollShadow
				onscroll={handleScroll}
			>
				<div>
					<!-- Staged changes -->
					<FileItems
						files={[]}
						folders={stagedFolder}
						databases={[]}
						databaseItems={[]}
						depth={0}
						parentIds={[]}
						ctx="git"
					/>

					<!-- Unstaged changes -->
					<FileItems
						files={[]}
						folders={unstagedFolder}
						databases={[]}
						databaseItems={[]}
						depth={0}
						parentIds={[]}
						ctx="git"
					/>
				</div>
			</div>
		{/if}
	{:else}
		<div class="section space x y">
			<p class="section-title">Not a Git repository</p>
			<p class="hint">
				This folder is not under version control. Run <code>git init</code>, or clone an existing
				repository and open that folder instead. The built-in terminal (<code>Ctrl+`</code>) is a
				good place to do it.
			</p>
		</div>
	{/if}
</div>

<style>
	.github-panel {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm-md);

		height: 100%;
		overflow: hidden;
	}

	.section {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.overflow-x-only {
		overflow-x: hidden;
		overflow-y: auto;
		overscroll-behavior-y: none;
	}

	.space.x {
		padding-left: var(--space-sm-md);
		padding-right: var(--space-sm-md);
	}
	.space.y {
		padding-top: var(--space-sm-md);
		padding-bottom: var(--space-sm-md);
	}

	.section-title {
		font-size: var(--fs-xs);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--gray-800);
	}

	.hint {
		font-size: var(--fs-xs);
		color: var(--gray-800);
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-sm);
	}

	.actions {
		margin-top: var(--space-xs);
		display: flex;
		gap: var(--space-xs);
	}

	:global(.github-panel .title-actions button:first-of-type) {
		margin-left: auto;
	}
	:global(.github-panel .title-actions button) {
		visibility: hidden;
	}
	:global(.github-panel .title-actions:hover button) {
		visibility: visible;
	}
</style>
