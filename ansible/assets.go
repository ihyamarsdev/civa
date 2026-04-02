package ansible

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed main.yml all:collections
var files embed.FS

func ReadFile(path string) ([]byte, error) {
	return files.ReadFile(path)
}

func Materialize(targetDir string) error {
	return fs.WalkDir(files, ".", func(assetPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		content, err := files.ReadFile(assetPath)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, filepath.FromSlash(assetPath))
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, content, 0o644); err != nil {
			return err
		}

		return nil
	})
}

func HasEmbeddedPlaybook() bool {
	_, err := files.ReadFile("main.yml")
	return err == nil
}

func HasEmbeddedTemplate(path string) bool {
	_, err := files.ReadFile(path)
	return err == nil
}
