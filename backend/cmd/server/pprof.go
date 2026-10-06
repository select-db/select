package main

import (
	"backend/db"

	"github.com/selectDb/toolkit"
)

func startPprofServer() {
	toolkit.RegisterStats("db", db.Stats)
	toolkit.StartPprofServer()
}
