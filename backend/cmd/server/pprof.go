package main

import (
	"backend/db"

	"github.com/selectDb/toolkit"
)

func startPprofServer() {
	// what the backend says about its database, read only when /debug/stats is
	toolkit.RegisterStats("db", db.Stats)
	toolkit.StartPprofServer()
}
