package fs_provider

import "os"

func (fsp *FSProvider) Stat(uri string) (FileStat, error) {
	path, err := fsp.GetOSPathFromURI(uri)
	if err != nil {
		return FileStat{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return FileStat{}, err
	}

	return fileInfoToStat(info), nil
}
