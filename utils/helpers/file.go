package helpers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/samber/lo"
)

func RemoveFilesAndFoldersInFolder(folder string, excludeFiles []string) error {
	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded files
		if !info.IsDir() && lo.Contains(excludeFiles, info.Name()) {
			return nil
		}

		// Remove files
		if !info.IsDir() {
			fmt.Printf("Removing file: %s\n", path)
			err = os.Remove(path)
			if err != nil {
				return err
			}
		}

		// Remove directories (after their contents have been removed)
		if info.IsDir() {
			isEmpty, err := isDirEmpty(path)
			if err != nil {
				return err
			}
			if isEmpty {
				fmt.Printf("Removing folder: %s\n", path)
				err = os.Remove(path)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	return err
}

// Helper function to check if a directory is empty
func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdir(1) // Try to read just one entry
	if err == io.EOF {
		return true, nil // Directory is empty
	}
	return false, err
}

func WriteToFile(filename, content string) (*os.File, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	_, err = file.WriteString(content)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func CreateFolder(folderpath string) error {
	if _, err := os.Stat(folderpath); os.IsNotExist(err) {
		err = os.MkdirAll(folderpath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

func CreateFile(filepath string) (*os.File, error) {
	file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	return file, nil
}
