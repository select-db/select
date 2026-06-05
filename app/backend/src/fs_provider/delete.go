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

	return nil
}
