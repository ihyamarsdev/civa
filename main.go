package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed scripts/civa
var embeddedScripts embed.FS

func main() {
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: bash is required to run civa")
		os.Exit(1)
	}

	scriptContent, err := embeddedScripts.ReadFile("scripts/civa")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load embedded civa script: %v\n", err)
		os.Exit(1)
	}

	tempDir, err := os.MkdirTemp("", "civa-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	scriptPath := filepath.Join(tempDir, "civa.sh")
	if err := os.WriteFile(scriptPath, scriptContent, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write embedded civa script: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(bashPath, append([]string{scriptPath}, os.Args[1:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}

		fmt.Fprintf(os.Stderr, "Error: failed to run civa: %v\n", err)
		os.Exit(1)
	}
}
