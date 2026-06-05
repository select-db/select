package fs_provider

import "os"

type MkdirParams struct {
	URI string `json:"uri"`
}

func (fsp *FSProvider) Mkdir(params MkdirParams) error {
	path, err := fsp.GetOSPathFromURI(params.URI)
	if err != nil {
		return err
	}
	return os.MkdirAll(path, 0o700)
}
