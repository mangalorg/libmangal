package libmangal

import (
	"github.com/spf13/afero"
	"io"
	"path/filepath"
)

// mergeDirectories merges two directories recursively from different filesystems.
// If a file exists in both directories it will be overwritten.
func mergeDirectories(
	dstFS afero.Fs, dstDir string,
	srcFS afero.Fs, srcDir string,
) error {
	srcFiles, err := afero.ReadDir(srcFS, srcDir)
	if err != nil {
		return err
	}

	for _, srcFile := range srcFiles {
		srcFilePath := filepath.Join(srcDir, srcFile.Name())
		dstFilePath := filepath.Join(dstDir, srcFile.Name())

		if srcFile.IsDir() {
			if err := mergeDirectories(
				dstFS, dstFilePath,
				srcFS, srcFilePath,
			); err != nil {
				return err
			}

			continue
		}

		exists, err := afero.Exists(dstFS, dstFilePath)
		if err != nil {
			return err
		}

		if exists {
			if err := dstFS.Remove(dstFilePath); err != nil {
				return err
			}
		} else {
			if err := dstFS.MkdirAll(filepath.Dir(dstFilePath), modeDir); err != nil {
				return err
			}
		}

		srcFile, err := srcFS.Open(srcFilePath)
		if err != nil {
			return err
		}

		dstFile, err := dstFS.Create(dstFilePath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(dstFile, srcFile); err != nil {
			return err
		}

		_ = srcFile.Close()
		_ = dstFile.Close()
	}

	return nil
}
