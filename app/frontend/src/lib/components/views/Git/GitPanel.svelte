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
		LinkExistingRepo,
		CompleteLinkExistingRepo,
		PushWorkspaceRepo,
		PushForceWithLease,
		CommitChanges,
		PullWorkspaceRepo,
		PullWithRebase,
		ResetBranchToRemote
	} from '$lib/wailsjs/go/git/Git';
	import LinkOptionsModal from '$lib/components/views/Git/LinkOptionsModal.svelte';

	import { graph } from '$lib/wailsjs/go/models';
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

	let remoteUrl = $state('');
	let commitMessage = $state('');

	const linkRemote = async () => {
		const status = await must(tryCatch(LinkExistingRepo, { remoteUrl }));
		if (status.scenario === 'checkout') {
			modalStore.set({
				content: (() => LinkOptionsModal) as () => Component,
				width: 400,
				props: {
					branch: status.branch,
					onChoice: async (choice: 'checkout' | 'keep') => {
						await must(tryCatch(CompleteLinkExistingRepo, choice));
						notify({ type: AlertType.Success, message: 'Remote linked' });
						await loadGitStatus();
					}
				}
			});
			return;
		}
		notify({ type: AlertType.Success, message: 'Remote linked' });
		await loadGitStatus();
	};

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
		return new graph.FolderNode({
			id,
			uri: id,
			type: 'folder',
			name,
			folder_id: '',
			files,
			folders: [],
			db_instances: [],
			badges: [files.length]
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

<div class="github-panel">
	{#if !workspaceGitStatus}
		<div class="section space x y">
			<Loader />
		</div>
	{:else if !workspaceGitStatus?.gitAvailable}
		<div class="section space x y">
			<p class="section-title">Git not available</p>
			{#if workspaceGitStatus?.configuredRemoteUrl}
				<p class="hint">
					This workspace is linked to a Git repository, but its files can’t sync
					because Git is not installed or not available in your PATH.<br />
					Install Git, then reopen the workspace to sync.
				</p>
			{:else}
				<p class="hint">
					Git is not installed or not available in your PATH. <br />
					Install Git to enable GitHub integration.
				</p>
			{/if}
		</div>
	{:else if workspaceGitStatus.isGitRepo && workspaceGitStatus.hasRemote}
		<!-- Staging and commit UI when linked -->
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
		<div class="section space x y" style="border-bottom: var(--border)">
			<div class="status-header">
				<p class="section-title">Current status</p>
			</div>

			{#if !workspaceGitStatus.isGitRepo}
				<p class="hint">This workspace is not yet a Git repository.</p>
			{:else if workspaceGitStatus.isGitRepo && !workspaceGitStatus.hasRemote}
				<p class="hint">Git repository detected with no configured remote.</p>
			{/if}
		</div>

		<div class="section space x">
			<p class="section-title">Link repository</p>

			<div class="field">
				<p class="label">Remote URL</p>
				<Input
					bind:value={remoteUrl}
					placeholder="git@github.com:owner/repo.git or https://github.com/owner/repo.git"
				/>
			</div>

			<div class="actions">
				<Button
					content={workspaceGitStatus.isGitRepo ? 'Update remote' : 'Link repository'}
					emphasis="high"
					onclick={linkRemote}
					size="sm"
					disabled={!remoteUrl}
				/>
			</div>
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

	.status-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-xs);
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

	.label {
		color: var(--gray-800);
		font-size: var(--fs-xs);
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
