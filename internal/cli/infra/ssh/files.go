package ssh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RewriteKnownHostEntry(host string, port int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return nil
	}

	return RewriteKnownHostEntryInHome(homeDir, host, port)
}

func RewriteKnownHostEntryInHome(homeDir, host string, port int) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	content, err := os.ReadFile(knownHostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read known_hosts at %s: %w", knownHostsPath, err)
	}
	newlineAtEnd := strings.HasSuffix(string(content), "\n")

	portHost := fmt.Sprintf("[%s]:%d", host, port)
	lines := strings.Split(string(content), "\n")
	updated := make([]string, 0, len(lines))
	changed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			updated = append(updated, line)
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			updated = append(updated, line)
			continue
		}

		hosts := strings.Split(fields[0], ",")
		kept := make([]string, 0, len(hosts))
		for _, item := range hosts {
			if item == host || (port > 0 && item == portHost) {
				changed = true
				continue
			}
			kept = append(kept, item)
		}

		if len(kept) == len(hosts) {
			updated = append(updated, line)
			continue
		}

		if len(kept) == 0 {
			continue
		}

		fields[0] = strings.Join(kept, ",")
		updated = append(updated, strings.Join(fields, " "))
	}

	if !changed {
		return nil
	}

	result := strings.Join(updated, "\n")
	if newlineAtEnd && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	permissions := os.FileMode(0o600)
	if info, statErr := os.Stat(knownHostsPath); statErr == nil {
		permissions = info.Mode().Perm()
	}

	if err := os.WriteFile(knownHostsPath, []byte(result), permissions); err != nil {
		return fmt.Errorf("failed to update known_hosts at %s: %w", knownHostsPath, err)
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
