package fs_provider

import "os"

type DeleteParams struct {
	URI       string `json:"uri"`
	Recursive bool   `json:"recursive"`
}

func (fsp *FSProvider) Delete(params DeleteParams) error {
	path, err := fsp.GetOSPathFromURI(params.URI)
	if err != nil {
		return err
	}

	if params.Recursive {
		return os.RemoveAll(path)
	}

	// Check if file exists before attempting to remove
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// File doesn't exist, nothing to delete, no error
		return nil
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	// A file's result metadata is a sidecar with no entry of its own in the
	// tree, so nothing else would ever remove it. Rename already moves it with
	// the file it belongs to; deleting is the same rule from the other side.
	_ = os.Remove(path + MetadataFileSuffix)

	return nil
}
