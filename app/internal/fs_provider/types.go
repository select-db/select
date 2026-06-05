package fs_provider

import "time"

type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeFile
	FileTypeDirectory
)

// FileStat contains basic metadata about a filesystem entry.
type FileStat struct {
	Type  FileType
	Size  int64
	Mtime time.Time
}

// DirEntry describes a single entry in a directory listing.
type DirEntry struct {
	Name string
	Type FileType
}
