// Command seesweep generates SQL that reads a hidden column every way it can
// and reports what got an answer about it. The shared see cases are the
// regression net; this is what finds the cases to add to them.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/selectDb/dialect/core/testutil"
	"github.com/selectDb/dialect/engine"
)

// hidden is the column GetSeeTestPermissions denies see on, named here only
// for the report.
const hidden = "main.users.email"

func main() {
	out := flag.String("out", "", "write the generated statements to this file as JSON and stop")
	in := flag.String("in", "", "read statements from this JSON file instead of generating them")
	examples := flag.Int("examples", 8, "how many failing statements to print")
	flag.Parse()

	statements, err := load(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *out != "" {
		if err := write(*out, statements); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %d statements to %s\n", len(statements), *out)
		return
	}
	if err := sweep(statements, *examples); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func load(path string) ([]string, error) {
	if path == "" {
		return generate(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var statements []string
	if err := json.Unmarshal(raw, &statements); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return statements, nil
}

func write(path string, statements []string) error {
	raw, err := json.MarshalIndent(statements, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// generate combines the ways a statement can name a relation, return a column
// and test one. Not every combination is valid SQLite; the sweep reports those
// separately rather than filtering them out, since what a dialect rejects is
// worth seeing too.
func generate() []string {
	sources := []string{
		"users",
		"main.users",
		`"users"`,
		"users u",
		"(SELECT email FROM users) q",
		"(SELECT email AS e FROM users) q",
		"(SELECT * FROM users) q",
		"users JOIN contacts ON users.id = contacts.id",
		"users u JOIN contacts c USING (email)",
		"users u NATURAL JOIN contacts c",
	}
	projections := []string{
		"*", "email", "users.email", "u.email", "q.email", "q.e", "q.*",
		"email AS x", "DISTINCT email", "lower(email)", "email || ''",
		"CASE WHEN id = 1 THEN email ELSE 'x' END",
		"(SELECT email FROM users LIMIT 1) AS s",
		"max(email)", "id, email", "id",
		"count(*) FILTER (WHERE email LIKE 'S%') AS n",
		"row_number() OVER (ORDER BY email) AS rn",
		"`email`", "[email]",
	}
	clauses := []string{
		"",
		"WHERE email LIKE 'S%'",
		"WHERE email LIKE 'z%'",
		"WHERE id IN (SELECT id FROM users WHERE email > '')",
		"WHERE EXISTS (SELECT 1 FROM contacts c WHERE c.email = users.email)",
		"GROUP BY email",
		"GROUP BY id HAVING max(email) > 'm'",
		"ORDER BY email",
		"ORDER BY email DESC LIMIT 1",
		"ORDER BY (SELECT email FROM users LIMIT 1)",
	}
	writes := []string{
		"INSERT INTO contacts (email) SELECT email FROM users",
		"INSERT INTO contacts (id, email) VALUES (9, (SELECT email FROM users LIMIT 1))",
		"UPDATE contacts SET email = (SELECT email FROM users LIMIT 1)",
		"UPDATE users SET age = 1 WHERE email LIKE 'S%'",
		"DELETE FROM users WHERE email LIKE 'S%'",
		"UPDATE users SET age = 1 WHERE id = 1 RETURNING email",
		"SELECT email FROM users UNION SELECT 'SECRET-alice'",
		"SELECT email FROM users UNION ALL SELECT 'SECRET-alice'",
		"SELECT email FROM users INTERSECT SELECT 'SECRET-alice'",
		"SELECT email FROM users EXCEPT SELECT 'SECRET-alice'",
	}

	seen := map[string]bool{}
	var statements []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if !seen[s] {
			seen[s] = true
			statements = append(statements, s)
		}
	}
	for _, source := range sources {
		for _, projection := range projections {
			for _, clause := range clauses {
				add(fmt.Sprintf("SELECT %s FROM %s %s", projection, source, clause))
			}
		}
	}
	for _, statement := range writes {
		add(statement)
	}
	sort.Strings(statements)
	return statements
}

// sweep runs every statement twice, against two databases differing only in
// the hidden column, and reports what got through.
func sweep(statements []string, examples int) error {
	var leaks, oracles, refused, ran, invalid int
	var leaked, answered []string

	for _, statement := range statements {
		first, err := observe(statement, "SECRET-alice", "SECRET-bob")
		if err != nil {
			return err
		}
		switch first.Outcome {
		case testutil.SeeRefused:
			refused++
			continue
		case testutil.SeeInvalid:
			invalid++
			continue
		}
		ran++
		if first.Leaked {
			leaks++
			if len(leaked) < examples {
				leaked = append(leaked, statement)
			}
			continue
		}
		second, err := observe(statement, "zzz-1", "zzz-2")
		if err != nil {
			return err
		}
		if second.Rows != first.Rows {
			oracles++
			if len(answered) < examples {
				answered = append(answered, fmt.Sprintf("%s\n      one: %.70s\n      two: %.70s",
					statement, first.Rows, second.Rows))
			}
		}
	}

	fmt.Printf("\n%d statements | ran %d | refused %d | not valid SQLite %d\n",
		len(statements), ran, refused, invalid)
	fmt.Printf("LEAKS %d, values of %s in a row\n", leaks, hidden)
	for _, statement := range leaked {
		fmt.Printf("  LEAK   %s\n", statement)
	}
	fmt.Printf("ORACLES %d, answering differently when only %s differs\n", oracles, hidden)
	for _, statement := range answered {
		fmt.Printf("  ORACLE %s\n", statement)
	}
	if leaks > 0 || oracles > 0 {
		return fmt.Errorf("%d leaks and %d oracles", leaks, oracles)
	}
	return nil
}

// observe runs one statement against a database holding the two given values
// in the hidden column, and reports what came back.
func observe(statement, first, second string) (answer testutil.SeeAnswer, err error) {
	defer func() {
		// A statement the inspectors stumble over must not take the sweep
		// down: that is a result too.
		if recovered := recover(); recovered != nil {
			answer, err = testutil.SeeAnswer{Outcome: testutil.SeeInvalid}, nil
		}
	}()

	db, meta, err := testutil.SeeOracleDB(first, second)
	if err != nil {
		return testutil.SeeAnswer{}, err
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := engine.ExecuteLocal(ctx,
		engine.Conn{DB: db, Meta: meta, Perms: testutil.GetSeeTestPermissions()},
		engine.DBInstance{ID: testutil.TestDBInstanceID, DBType: "sqlite"}, statement, engine.Options{})
	return testutil.ReadSeeAnswer(result.RowCount, result.Rows, result.Errors), nil
}
