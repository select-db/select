package scripts

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

func DumpSchema(db *sql.DB, outputFile string) error {
	var schemaStatements []string

	// Add CREATE SCHEMA statements
	schemaRows, err := db.Query(`
		SELECT DISTINCT schemaname
		FROM pg_tables
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY schemaname;
	`)
	if err != nil {
		return fmt.Errorf("failed to fetch schema names: %w", err)
	}
	defer func() { _ = schemaRows.Close() }()

	for schemaRows.Next() {
		var schema string
		if err := schemaRows.Scan(&schema); err != nil {
			return fmt.Errorf("failed to scan schema name: %w", err)
		}
		schemaStatements = append(schemaStatements, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %q;", schema))
	}
	if err := schemaRows.Err(); err != nil {
		return fmt.Errorf("error iterating schema rows: %w", err)
	}

	// Add CREATE TABLE statements
	tableRows, err := db.Query(`
		SELECT 'CREATE TABLE ' || quote_ident(schemaname) || '.' || quote_ident(tablename) || E' (\n' ||
		       string_agg('  ' || quote_ident(column_name) || ' ' || data_type, E',\n') || E'\n);'
		FROM (
			SELECT 
				schemaname, tablename, column_name,
				CASE
					WHEN data_type = 'ARRAY' THEN udt_name
					ELSE data_type
				END AS data_type
			FROM pg_tables
			JOIN information_schema.columns 
			  ON tablename = table_name AND schemaname = table_schema
			WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
			ORDER BY schemaname, tablename, ordinal_position
		) AS column_defs
		GROUP BY schemaname, tablename
		ORDER BY schemaname, tablename;
	`)
	if err != nil {
		return fmt.Errorf("failed to query table schemas: %w", err)
	}
	defer func() { _ = tableRows.Close() }()

	for tableRows.Next() {
		var createStmt string
		if err := tableRows.Scan(&createStmt); err != nil {
			return fmt.Errorf("failed to scan create table statement: %w", err)
		}
		schemaStatements = append(schemaStatements, createStmt)
	}
	if err := tableRows.Err(); err != nil {
		return fmt.Errorf("error iterating table rows: %w", err)
	}

	// Write to file
	schemaDump := strings.Join(schemaStatements, "\n\n") + "\n"
	if err := os.WriteFile(outputFile, []byte(schemaDump), 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	return nil
}
