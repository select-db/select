package core

// UnknownStatement is what an inspector returns for a statement it parsed but
// cannot classify. Permission checks read it as manage, so a statement nobody
// has taught us to read is refused rather than waved through as nothing.
func UnknownStatement() InspectStatement {
	return InspectStatement{Operation: InspectOpUnknown}
}

// OrUnknown returns *stmt, or an unknown statement when stmt is nil. Inspectors
// call it at the one point where a parsed statement becomes a result, so a
// dispatcher that falls through cannot drop the statement on the floor.
func OrUnknown(stmt *InspectStatement) InspectStatement {
	if stmt == nil {
		return UnknownStatement()
	}
	return *stmt
}
