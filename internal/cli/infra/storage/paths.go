package storage

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const userCivaHomeDirectory = "~/.civa"

func CivaHomeDirectory() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		homeDir = strings.TrimSpace(os.Getenv("HOME"))
	}
	if homeDir == "" {
		if currentUser, userErr := user.Current(); userErr == nil {
			homeDir = strings.TrimSpace(currentUser.HomeDir)
		}
	}
	if homeDir == "" {
		return filepath.Clean(strings.TrimPrefix(userCivaHomeDirectory, "~"))
	}

	return filepath.Join(homeDir, strings.TrimPrefix(userCivaHomeDirectory, "~/"))
}

func RunRootDirectoryPath() string {
	return filepath.Join(CivaHomeDirectory(), "runs")
}

func LatestPlanPointerFilePath() string {
	return filepath.Join(CivaHomeDirectory(), "latest-plan")
}

func PlanPathForName(name string) string {
	return filepath.Join(RunRootDirectoryPath(), name, "plan.md")
}

func PlanDirForName(name string) string {
	return filepath.Join(RunRootDirectoryPath(), name)
}

func ConfigDirectoryPath() string {
	return filepath.Join(CivaHomeDirectory(), "config")
}

func WebServerConfigFilePath() string {
	return filepath.Join(ConfigDirectoryPath(), "web-server.json")
}
