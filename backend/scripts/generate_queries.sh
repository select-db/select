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
if ! find "$ROOT_DIR" -type f \( -name "*_query.sql" -o -name "*_statement.sql" \) -exec sh -c 'cat {} && echo "\n"' \; > "$TEMP_SQL_FILE"; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} Failed to find or concatenate queries.sql files"
    exit 1
fi

# Run sqlc generate
echo -e "${BOLD}[Generate]${NORMAL} Running sqlc generate..."
if ! sqlc generate; then
    echo -e "${RED}${BOLD}[Error]${NORMAL}${NC} sqlc generate failed"
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