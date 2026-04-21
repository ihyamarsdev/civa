package infra

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintUsageUsesInteractiveHelpSelection(t *testing.T) {
	origPromptFn := interactiveHelpPromptFn
	origTerminalFn := interactiveHelpIsTerminalFn
	t.Cleanup(func() {
		interactiveHelpPromptFn = origPromptFn
		interactiveHelpIsTerminalFn = origTerminalFn
	})

	interactiveHelpIsTerminalFn = func(_ int) bool { return true }
	interactiveHelpPromptFn = func(options []helpMenuOption) (string, error) {
		if len(options) == 0 {
			t.Fatal("expected interactive help options")
		}
		return commandPlan, nil
	}

	output := captureStdout(t, func() {
		printUsage(false)
	})

	if !strings.Contains(output, "civa plan") {
		t.Fatalf("expected plan help output, got %q", output)
	}
	if strings.Contains(output, "auth cloudflare           Manage Cloudflare auth profiles") {
		t.Fatalf("expected interactive help to avoid full overview, got %q", output)
	}
}

func TestPrintUsageFallsBackToOverviewWhenNonInteractive(t *testing.T) {
	origPromptFn := interactiveHelpPromptFn
	origTerminalFn := interactiveHelpIsTerminalFn
	t.Cleanup(func() {
		interactiveHelpPromptFn = origPromptFn
		interactiveHelpIsTerminalFn = origTerminalFn
	})

	promptCalled := false
	interactiveHelpIsTerminalFn = func(_ int) bool { return true }
	interactiveHelpPromptFn = func(_ []helpMenuOption) (string, error) {
		promptCalled = true
		return commandPlan, nil
	}

	output := captureStdout(t, func() {
		printUsage(true)
	})

	if promptCalled {
		t.Fatal("expected non-interactive usage to skip prompt")
	}
	if !strings.Contains(output, "civa <command> [options]") {
		t.Fatalf("expected full usage overview, got %q", output)
	}
	if !strings.Contains(output, "auth cloudflare           Manage Cloudflare auth profiles") {
		t.Fatalf("expected commands overview in output, got %q", output)
	}
	if !strings.Contains(output, "bootstrap                 Simplified onboarding: setup, doctor, and config") {
		t.Fatalf("expected bootstrap overview in output, got %q", output)
	}
	if !strings.Contains(output, "deploy                    Simplified delivery flow: plan, apply, and custom run") {
		t.Fatalf("expected deploy overview in output, got %q", output)
	}
	if !strings.Contains(output, "ops                       Operational tools: playbook, secret, auth, and provider tools") {
		t.Fatalf("expected ops overview in output, got %q", output)
	}
	if !strings.Contains(output, "playbook run [plan-name]   Run managed/local custom playbook on existing plan artifacts") {
		t.Fatalf("expected playbook command overview in output, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	origStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer

	readDone := make(chan string, 1)
	go func() {
		var buffer bytes.Buffer
		_, _ = io.Copy(&buffer, reader)
		readDone <- buffer.String()
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = origStdout

	output := <-readDone
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return output
}
