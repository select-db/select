package syncer

import (
	"fmt"
	"selectDb/backend/db/generated"
)

func (s *Syncer) pushCommitToFrontend(commit generated.MutationCommit) error {
	err := s.Graph.Mutate(s.ctx, commit)
	if err != nil {
		return fmt.Errorf("pushMutationToFrontend: failed to apply mutation (operation=%s, table=%s, args=%v): %w",
			commit.Operation, commit.TableName, commit.Payload, err)
	}
	return nil
}
