package fs_provider

import "time"

// MetadataFileSuffix names the sidecar a query file carries its result
// metadata in. It has no entry of its own in the workspace tree, so every
// operation on a file has to carry it: Rename moves it, Delete removes it.
const MetadataFileSuffix = ".metadata.json"

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
