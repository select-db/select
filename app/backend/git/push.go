package git


// PushWorkspaceRepo pushes the current branch of the workspace repository to
// its "origin" remote, creating the upstream tracking branch if needed.
func (g *Git) PushWorkspaceRepo() error {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return err
	}

	branch, err := getCurrentBranch(ctx, root)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "push", "-u", "origin", branch); err != nil {
		return err
	}

	return nil
}

// PushForceWithLease pushes the current branch to origin using --force-with-lease,
// overwriting the remote only if it has not been updated since the last fetch.
func (g *Git) PushForceWithLease() error {
	ctx := g.context()

	root, err := g.prepareGit(ctx)
	if err != nil {
		return err
	}

	branch, err := getCurrentBranch(ctx, root)
	if err != nil {
		return err
	}

	if err := runGit(ctx, root, "push", "--force-with-lease", "-u", "origin", branch); err != nil {
		return err
	}

	return nil
}
