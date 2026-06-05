package core

import (
	"regexp"
	"strings"
)

// FormatTableDDL formats a CREATE TABLE DDL statement with aligned columns
func FormatTableDDL(ddl string) string {
	if ddl == "" {
		return ddl
	}

	lines := strings.Split(ddl, "\n")
	if len(lines) < 2 {
		return ddl
	}

	// Find CREATE TABLE line
	createIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "CREATE TABLE") {
			createIdx = i
			break
		}
	}
	if createIdx == -1 {
		return ddl
	}

	// Extract column lines - match lines with indentation followed by column definition
	// Pattern: spaces + (quoted identifier OR unquoted identifier) + space + type + constraints + comma
	var colLines []string
	var constraints []string
	
	// Match quoted identifiers: "identifier" or unquoted: identifier
	// Then space, then rest of the line (type, constraints, comma)
	colRegex := regexp.MustCompile(`^(\s+)(("[^"]+")|[^\s"]+)\s+(.+)$`)

	for i := createIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if trimmed == ")" || trimmed == ");" {
			break
		}
		if strings.HasPrefix(trimmed, "CONSTRAINT") {
			constraints = append(constraints, lines[i])
			continue
		}
		if colRegex.MatchString(lines[i]) {
			colLines = append(colLines, lines[i])
		}
	}

	if len(colLines) == 0 {
		return ddl
	}

	// Parse columns and calculate max widths
	type col struct {
		indent      string
		name        string
		typeAndRest string
	}
	var cols []col
	maxNameLen, maxTypeLen := 0, 0

	for _, line := range colLines {
		match := colRegex.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}
		indent, name, typeAndRest := match[1], match[2], match[4]
		
		// Remove trailing comma
		typeAndRest = strings.TrimSuffix(strings.TrimSpace(typeAndRest), ",")
		
		// Extract type (before NOT NULL, DEFAULT)
		typeEnd := len(typeAndRest)
		for _, sep := range []string{" NOT NULL", " DEFAULT"} {
			if idx := strings.Index(typeAndRest, sep); idx != -1 && idx < typeEnd {
				typeEnd = idx
			}
		}
		colType := strings.TrimSpace(typeAndRest[:typeEnd])

		if len(name) > maxNameLen {
			maxNameLen = len(name)
		}
		if len(colType) > maxTypeLen {
			maxTypeLen = len(colType)
		}

		cols = append(cols, col{indent, name, typeAndRest})
	}

	// Rebuild DDL
	var result strings.Builder
	result.WriteString(lines[createIdx])
	result.WriteString("\n")
	
	// Format columns
	for _, c := range cols {
		typeEnd := len(c.typeAndRest)
		for _, sep := range []string{" NOT NULL", " DEFAULT"} {
			if idx := strings.Index(c.typeAndRest, sep); idx != -1 && idx < typeEnd {
				typeEnd = idx
			}
		}
		colType := strings.TrimSpace(c.typeAndRest[:typeEnd])
		rest := strings.TrimSpace(c.typeAndRest[typeEnd:])
		
		result.WriteString(c.indent)
		result.WriteString(padRight(c.name, maxNameLen))
		result.WriteString(" ")
		if rest != "" {
			result.WriteString(padRight(colType, maxTypeLen))
			result.WriteString(" ")
			result.WriteString(rest)
		} else {
			result.WriteString(colType)
		}
		result.WriteString(",")
		result.WriteString("\n")
	}

	// Add constraints with blank line separator
	if len(constraints) > 0 {
		result.WriteString("\n")
		for _, constraint := range constraints {
			result.WriteString(constraint)
			result.WriteString("\n")
		}
	}

	// Add closing paren
	for i := createIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == ")" || trimmed == ");" {
			result.WriteString(lines[i])
			break
		}
	}

	return result.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
