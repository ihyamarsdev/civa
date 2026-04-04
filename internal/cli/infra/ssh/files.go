package ssh

import (
	"fmt"
	"os"
	"path/filepath"
)

func RotateKnownHostsFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return nil
	}

	return RotateKnownHostsFileInHome(homeDir)
}

func RotateKnownHostsFileInHome(homeDir string) error {
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect known_hosts at %s: %w", knownHostsPath, err)
	}

	backupPath := knownHostsPath + ".old"
	if err := os.Rename(knownHostsPath, backupPath); err != nil {
		return fmt.Errorf("failed to rotate known_hosts to %s: %w", backupPath, err)
	}

	return nil
}

func EnsureUserCivaDirectoryInHome(homeDir string) (string, error) {
	userCivaDir := filepath.Join(homeDir, ".civa")
	if err := os.MkdirAll(userCivaDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create user civa directory %s: %w", userCivaDir, err)
	}

	return userCivaDir, nil
}
