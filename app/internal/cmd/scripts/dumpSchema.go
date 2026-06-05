package scripts

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// DumpSchemaFromDB writes the database schema to the specified output file.
// `db` must be an open *sql.DB connection.
func DumpSchema(db *sql.DB, outputFile string) error {
	// Query all CREATE statements from sqlite_master except for sqlite_sequence table (used for autoincrement)
	rows, err := db.Query(`
		SELECT type, name, tbl_name, sql 
		FROM sqlite_master 
		WHERE sql IS NOT NULL AND type IN ('table', 'index', 'trigger', 'view')
		ORDER BY type, name
	`)
	if err != nil {
		return fmt.Errorf("failed to query schema: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var schemaStatements []string

	for rows.Next() {
		var objectType, name, tblName, sqlStmt string
		if err := rows.Scan(&objectType, &name, &tblName, &sqlStmt); err != nil {
			return fmt.Errorf("failed to scan schema row: %w", err)
		}
		// Skip sqlite_sequence or other internal tables if you want:
		if name == "sqlite_sequence" {
			continue
		}

		// Append the statement (with semicolon) to schema list
		schemaStatements = append(schemaStatements, sqlStmt+";")
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating schema rows: %w", err)
	}

	// Join all schema statements with double newlines for readability
	schemaDump := strings.Join(schemaStatements, "\n\n") + "\n"

	// Write to output file
	if err := os.WriteFile(outputFile, []byte(schemaDump), 0644); err != nil {
		return fmt.Errorf("failed to write schema dump file: %w", err)
	}

	return nil
}
