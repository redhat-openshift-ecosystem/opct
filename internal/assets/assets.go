package assets

import (
	"embed"
	"io/fs"
)

var efs *embed.FS

func GetData() *embed.FS {
	return efs
}

func UpdateData(d *embed.FS) {
	efs = d
}

// GetAllFilenames return all file names from an path in embeded EFS.
func GetAllFilenames(efs *embed.FS, path string) (files []string, err error) {
	if err := fs.WalkDir(efs, path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		files = append(files, path)

		return nil
	}); err != nil {
		return nil, err
	}

	return files, nil
}
