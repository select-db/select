package syncer

import (
	"database/sql/driver"
	"testing"

	"selectDb/backend/db/generated"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestQueueMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	queries := generated.New(db)
	s := &Syncer{Queries: queries}

	t.Run("query contains @no-track", func(t *testing.T) {
		err := s.CommitMutation(&QueueMutationParams{
			Query: `; -- @no-track
			UPDATE file SET name='x';`,
			TableName: "file",
			Operation: "UPDATE",
			Args: []driver.NamedValue{
				{Name: "id", Value: "abc"},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("non-trackable table", func(t *testing.T) {
		err := s.CommitMutation(&QueueMutationParams{
			Query:     "UPDATE secret SET val='x'",
			TableName: "secret",
			Operation: "UPDATE",
			Args: []driver.NamedValue{
				{Name: "id", Value: "abc"},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("missing id in args", func(t *testing.T) {
		err := s.CommitMutation(&QueueMutationParams{
			Query:     "UPDATE role SET name='x'",
			TableName: "role",
			Operation: "UPDATE",
			Args:      []driver.NamedValue{},
		})
		assert.ErrorContains(t, err, "id not found")
	})

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExtractID(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		args := []driver.NamedValue{
			{Name: "id", Value: "123"},
			{Name: "name", Value: "foo"},
		}
		id, err := extractObjectID(args)
		assert.NoError(t, err)
		assert.Equal(t, "123", id)
	})

	t.Run("missing id", func(t *testing.T) {
		args := []driver.NamedValue{{Name: "name", Value: "foo"}}
		_, err := extractObjectID(args)
		assert.ErrorContains(t, err, "id not found")
	})

	t.Run("id not string", func(t *testing.T) {
		args := []driver.NamedValue{{Name: "id", Value: 123}}
		_, err := extractObjectID(args)
		assert.ErrorContains(t, err, "id is not a string")
	})
}

func TestBuildAndMergeArgs(t *testing.T) {
	t.Run("merge with empty existing", func(t *testing.T) {
		newArgs := []driver.NamedValue{
			{Name: "foo", Value: "bar"},
			{Name: "num", Value: 42},
		}
		b, err := buildAndMergeArgs(newArgs, nil)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"foo":"bar","num":42}`, string(b))
	})

	t.Run("merge with existing JSON", func(t *testing.T) {
		existing := []byte(`{"foo":"old","baz":1}`)
		newArgs := []driver.NamedValue{
			{Name: "foo", Value: "new"},
			{Name: "num", Value: 42},
		}
		b, err := buildAndMergeArgs(newArgs, existing)
		assert.NoError(t, err)
		assert.JSONEq(t, `{"foo":"new","baz":1,"num":42}`, string(b))
	})

	t.Run("error on unnamed arg", func(t *testing.T) {
		newArgs := []driver.NamedValue{{Name: "", Value: 1}}
		_, err := buildAndMergeArgs(newArgs, nil)
		assert.ErrorContains(t, err, "has no name")
	})

	t.Run("error on invalid existing JSON", func(t *testing.T) {
		_, err := buildAndMergeArgs(nil, []byte("{invalid"))
		assert.ErrorContains(t, err, "failed to unmarshal")
	})
}
