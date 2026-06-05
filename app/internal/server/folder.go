package server

import "strings"

// Colon is invalid in directory names on macOS. Use a reversible encoding.
const folderColonRepl = "._."

// DomainToFolderName returns a filesystem-safe folder name for the domain.
func DomainToFolderName(domain string) string {
	return strings.ReplaceAll(domain, ":", folderColonRepl)
}

// FolderNameToDomain reverses DomainToFolderName.
func FolderNameToDomain(folder string) string {
	return strings.ReplaceAll(folder, folderColonRepl, ":")
}
