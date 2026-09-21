// Command seesweep generates SQL that reads a hidden column every way it can
// and checks that none of it gets an answer about that column.
//
// The see cases in core are the regression net; this is the net that finds
// what to add to them. It builds statements by combining sources, projections
// and clauses, runs each against SQLite through the same path a query takes,
// and reports two things: a value of the hidden column reaching a row, and a
// statement answering differently on two databases that differ in nothing but
// that column, which is the same leak told one answer at a time.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/selectDb/dialect/core"
	"github.com/selectDb/dialect/engine"
)

const (
	instanceID = "db1"
	hidden     = "main.users.email"
)

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
		switch first.outcome {
		case outcomeRefused:
			refused++
			continue
		case outcomeInvalid:
			invalid++
			continue
		}
		ran++
		if first.leaked {
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
		if second.rows != first.rows {
			oracles++
			if len(answered) < examples {
				answered = append(answered, fmt.Sprintf("%s\n      one: %.70s\n      two: %.70s",
					statement, first.rows, second.rows))
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

type outcome int

const (
	outcomeRan outcome = iota
	outcomeRefused
	outcomeInvalid
)

type observation struct {
	outcome outcome
	rows    string
	leaked  bool
}

// observe runs one statement against a database holding the two given values
// in the hidden column, and reports what came back.
func observe(statement, first, second string) (obs observation, err error) {
	defer func() {
		// A statement the inspectors stumble over must not take the sweep
		// down: that is a result too.
		if recovered := recover(); recovered != nil {
			obs, err = observation{outcome: outcomeInvalid}, nil
		}
	}()

	db, meta, err := database(first, second)
	if err != nil {
		return observation{}, err
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result := engine.ExecuteLocal(ctx, engine.Conn{DB: db, Meta: meta, Perms: permissions()},
		engine.DBInstance{ID: instanceID, DBType: "sqlite"}, statement, engine.Options{})
	if len(result.Errors) > 0 {
		if strings.Contains(strings.Join(result.Errors, " "), "permission denied") {
			return observation{outcome: outcomeRefused}, nil
		}
		return observation{outcome: outcomeInvalid}, nil
	}

	var rows strings.Builder
	fmt.Fprintf(&rows, "n=%d|", result.RowCount)
	for _, row := range result.Rows {
		for _, value := range row {
			fmt.Fprintf(&rows, "%v,", value)
			if text, ok := value.(string); ok && strings.Contains(text, "SECRET") {
				obs.leaked = true
			}
		}
		rows.WriteString(";")
	}
	obs.rows = rows.String()
	return obs, nil
}

func database(first, second string) (*sql.DB, *core.Metadata, error) {
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
	meta := core.GetSeeTestMetadata()
	return db, &meta, nil
}

func permissions() core.CompiledPermissions {
	return core.GetSeeTestPermissions()
}
