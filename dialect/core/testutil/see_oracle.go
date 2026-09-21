package testutil

import (
	"database/sql"
	"fmt"
	"strings"

	core "github.com/selectDb/dialect/core"
)

// SeeOracleSentinel marks a value of the hidden column, so a run can tell a
// value that reached a row from one that did not.
const SeeOracleSentinel = "SECRET"

// SeeOutcome is what became of a statement.
type SeeOutcome int

const (
	SeeRan SeeOutcome = iota
	SeeRefused
	SeeInvalid
)

// SeeAnswer is what a statement reported. Rows renders the result so two runs
// can be compared: where they differ, the statement has answered about the
// column that differs between the two databases.
type SeeAnswer struct {
	Outcome SeeOutcome
	Rows    string
	Leaked  bool
}

// SeeOracleDB builds a database whose only difference from its twin is the
// value of the hidden column, alongside the metadata the cases describe.
func SeeOracleDB(first, second string) (*sql.DB, *core.Metadata, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY, email TEXT, age INTEGER);
		CREATE TABLE contacts(id INTEGER PRIMARY KEY, email TEXT);`); err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`INSERT INTO users VALUES (1,?,30),(2,?,25)`, first, second); err != nil {
		return nil, nil, err
	}
	if _, err := db.Exec(`INSERT INTO contacts VALUES (1,'carol@example.com')`); err != nil {
		return nil, nil, err
	}
	meta := GetSeeTestMetadata()
	return db, &meta, nil
}

// ReadSeeAnswer reads what a result carries. It takes the parts rather than
// the result itself, so the package the engine lives in can call it from its
// own tests.
func ReadSeeAnswer(rowCount int, rows [][]any, errs []string) SeeAnswer {
	if len(errs) > 0 {
		if strings.Contains(strings.Join(errs, " "), "permission denied") {
			return SeeAnswer{Outcome: SeeRefused, Rows: "refused"}
		}
		return SeeAnswer{Outcome: SeeInvalid, Rows: "not valid SQLite"}
	}
	answer := SeeAnswer{Outcome: SeeRan}
	var seen strings.Builder
	fmt.Fprintf(&seen, "rows=%d|", rowCount)
	for _, row := range rows {
		for _, value := range row {
			fmt.Fprintf(&seen, "%v,", value)
			if text, ok := value.(string); ok && strings.Contains(text, SeeOracleSentinel) {
				answer.Leaked = true
			}
		}
		seen.WriteString(";")
	}
	answer.Rows = seen.String()
	return answer
}
