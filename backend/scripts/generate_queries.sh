#!/bin/bash
set -euo pipefail  # Exit on error, treat unset variables as errors, and catch pipeline failures

# Text formatting
BOLD=$(tput bold)
NORMAL=$(tput sgr0)
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'  # No Color
CHECKMARK="\xE2\x9C\x94"

# Paths and filenames
ROOT_DIR="./"
GENERATED_DIR="${ROOT_DIR}/db/generated"
TEMP_SQL_FILE="${GENERATED_DIR}/all_queries.sql"
OUTPUT_SQL_GO="${GENERATED_DIR}/queries.sql.go"
GENERATED_TEMP_GO="${GENERATED_DIR}/all_queries.sql.go"

# Ensure sqlc is installed
if ! command -v sqlc &> /dev/null; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} ${BOLD}sqlc package not found...${NORMAL}"
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Doc:"
    echo -e "   https://docs.sqlc.dev/en/stable/overview/install.html"
    exit 1
fi

# Pin sqlc to a known-good version so generation stays reproducible and in step
# with the app module (which requires v1.31.1 for its SQLite queries).
REQUIRED_SQLC_VERSION="v1.31.1"
CURRENT_SQLC_VERSION="$(sqlc version)"
if [[ "$CURRENT_SQLC_VERSION" != "$REQUIRED_SQLC_VERSION" ]]; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} sqlc ${REQUIRED_SQLC_VERSION} required, found ${CURRENT_SQLC_VERSION}."
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Upgrade: ${BOLD}brew upgrade sqlc${NORMAL} (or) go install github.com/sqlc-dev/sqlc/cmd/sqlc@${REQUIRED_SQLC_VERSION}"
    exit 1
fi

# Find all .sql files and combine them into one file
echo -e "${BOLD}[Generate]${NORMAL} Looking for queries.sql files..."
# Sorted, so the concatenation order is the same on every machine. sqlc happens
# to sort its own output too, but the CI check diffs this file's result against
# what is committed, and that must not depend on filesystem order.
if ! find "$ROOT_DIR" -type f \( -name "*_query.sql" -o -name "*_statement.sql" \) | sort | while read -r file; do
    cat "$file"
    echo
done > "$TEMP_SQL_FILE"; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Failed to find or concatenate queries.sql files"
    exit 1
fi

# Run sqlc generate
echo -e "${BOLD}[Generate]${NORMAL} Running sqlc generate..."
if ! sqlc generate; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} sqlc generate failed"
    exit 1
fi

# A nullable column whose Postgres type has no matching override in sqlc.yml
# silently falls back to database/sql's own null types instead of the JSONNull*
# wrappers the API layer marshals. That is invisible in review, and it is how
# db/generated drifted before.
#
# It is easy to hit because sqlc matches db_type against the spelling the column
# was declared with, and the two ways of writing one type do not normalize to
# each other: TIMESTAMPTZ stays bare, TIMESTAMP WITH TIME ZONE becomes
# pg_catalog.timestamptz, and db/migrations uses both. Rather than chase every
# spelling, fail on the fallback itself -- it catches the whole class, including
# types with no override at all (numeric, date, int2).
#
# Both files are checked: models.go holds the table structs, the queries file
# holds the row and param structs a join produces, which no table struct covers.
if grep -n "sql\\.Null" "${GENERATED_DIR}/models.go" "$GENERATED_TEMP_GO"; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} generated code fell back to database/sql null types."
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Add an override for the column types listed above to sqlc.yml."
    rm -f "$TEMP_SQL_FILE" "$GENERATED_TEMP_GO"
    exit 1
fi

# Rename the generated file
echo -e "${BOLD}[Generate]${NORMAL} Cleaning..."
if ! mv "$GENERATED_TEMP_GO" "$OUTPUT_SQL_GO"; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Failed to rename generated file"
    exit 1
fi

rm -f "$TEMP_SQL_FILE"

echo -e "${GREEN}${BOLD}[Generate]${NORMAL}${NC} ${CHECKMARK} ${BOLD}Queries are ready to use${NORMAL}"
exit 0