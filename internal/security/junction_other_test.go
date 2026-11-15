//go:build !windows

package security

import (
	"os"
	"path/filepath"
)

func createTestReparseEscape(tRoot, linkName, targetDir, targetFile string) (string, error) {
	linkPath := filepath.Join(tRoot, linkName)
	if err := os.Symlink(targetFile, linkPath); err != nil {
		return "", err
	}
	return linkName, nil
}
