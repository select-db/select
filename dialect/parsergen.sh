#!/usr/bin/env bash
# Regenerates a dialect's ANTLR parser from its grammar, byte for byte the way
# the committed files were made, so CI can diff the two.
# Usage: dialect/parsergen.sh mysql|postgresql|sqlite ...
# The jars are cached in $ANTLR_JAR_DIR, fetched on first use or by --fetch,
# which the offline agent sandbox needs done ahead. Java is required.
set -euo pipefail

jars=${ANTLR_JAR_DIR:-$HOME/.cache/antlr}
declare -A sums=(
	[4.13.1]=bc13a9c57a8dd7d5196888211e5ede657cb64a3ce968608697e4f668251a8487
	[4.13.2]=eae2dfa119a64327444672aff63e9ec35a20180dc5b8090b7a6ab85125df4d76
)

fetch() {
	local jar="$jars/antlr-$1-complete.jar"
	echo "${sums[$1]}  $jar" | sha256sum -c --status 2>/dev/null && return
	mkdir -p "$jars"
	curl -fsSL --retry 5 --retry-all-errors -o "$jar.part" \
		"https://repo1.maven.org/maven2/org/antlr/antlr4/$1/antlr4-$1-complete.jar"
	echo "${sums[$1]}  $jar.part" | sha256sum -c --quiet
	mv "$jar.part" "$jar"
}

antlr() {
	fetch "$1"
	java -jar "$jars/antlr-$1-complete.jar" -Dlanguage=Go -package parser -visitor "${@:2}"
}

if [ "${1:-}" = --fetch ]; then
	for version in "${!sums[@]}"; do fetch "$version"; done
	exit 0
fi

root=$(cd "$(dirname "$0")" && pwd)
for dialect in "$@"; do
	cd "$root/$dialect/parser"
	case $dialect in
	mysql)
		antlr 4.13.2 MySQLLexer.g4 MySQLParser.g4
		gofmt -w .
		;;
	postgresql)
		antlr 4.13.2 PostgreSQLLexer.g4 PostgreSQLParser.g4
		rm -f ./*.interp ./*.tokens
		gofmt -w .
		sed -i 's|^package parser // PostgreSQL.*$|package parser|' postgresql_lexer.go postgresql_parser.go postgresqlparser_*.go
		;;
	sqlite)
		antlr 4.13.1 ./SQLiteLexer.g4 ./SQLiteParser.g4
		gofmt -w .
		perl -0pi -e 's/^(package parser[^\n]*\n)(?!\n)/$1\n/m' sqlite_lexer.go sqlite_parser.go sqliteparser_*.go
		;;
	*)
		echo "parsergen.sh: no grammar for $dialect" >&2
		exit 2
		;;
	esac
done
