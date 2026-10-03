// Package cellar is the cellar server: it holds the managed databases. It runs
// as its own process (`select-backend cellar`) or inside the backend with
// CELLAR=local, keeps each database's SQLite file on its disk and a copy in the
// bucket, and never reads Postgres. Everything here runs on the cellar; the
// backend reaches it through datasource/managed/cellarclient, after its own checks.
package cellar
