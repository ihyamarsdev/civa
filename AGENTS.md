# AGENTS.md

Guidance for coding agents working in `civa`.

## Project Snapshot
- Language: Go (`go 1.26`, module `civa`).
- Type: interactive CLI for VPS automation using embedded Ansible assets.
- Entry point: `main.go` -> `internal/cli.Run(...)`.
- Core package: `internal/cli`.
- Embedded assets: `ansible/` materialized at runtime.
- Release pipeline: `.goreleaser.yaml` + `.github/workflows/release.yml`.

## Operational Flow (Merged from AGENT.md)
- Primary flow: collect operator input -> generate run artifacts (`~/.civa/runs/<run-id>/`) -> review plan -> apply plan.
- This repository focuses on CLI orchestration and execution planning; provisioning logic is delegated to embedded Ansible assets.
- `civa setup` is the bootstrap command for first-time SSH key installation on a target host.
- `civa plan start` assumes key-based SSH access (password mode is not supported for planning).
- `preview` and `apply` operate on an existing generated plan (by plan name or `--plan-file`).

## Rule Sources (Cursor/Copilot)
- `.cursor/rules/`: not found.
- `.cursorrules`: not found.
- `.github/copilot-instructions.md`: not found.
- Use this file and existing repository patterns as the authoritative guide.

## Repo Areas You Will Touch Most
- `internal/cli/app.go`: command parsing, dispatch, config defaults/validation.
- `internal/cli/runtime.go`: runtime artifacts, doctor/setup helpers, ansible execution.
- `internal/cli/interactive.go`: interactive prompt flows and cancellation handling.
- `internal/cli/completion.go`: shell completion and hidden completion support.
- `internal/cli/app_test.go`: command/runtime tests and regression coverage.
- `docs/`: user-facing command and architecture documentation.

## Build / Test / Lint Commands
Run all commands from repo root (`/home/uya/project/civa`).

### Build
```bash
go build -o bin/civa .
```

### Quick smoke checks
```bash
./bin/civa help
./bin/civa version
```

### Run all tests
```bash
go test ./...
```

### Run tests in one package
```bash
go test ./internal/cli
```

### Run a single test function (important)
```bash
go test ./internal/cli -run '^TestResolveComponentsSupportsMixedTokens$'
```

### Run a single subtest
```bash
go test ./internal/cli -run 'TestParent/Subcase'
```

### Re-run without cache
```bash
go test ./internal/cli -run '^TestName$' -count=1
```

### Verbose single-test output
```bash
go test ./internal/cli -run '^TestName$' -v
```

### Lint/format status in this repository
- No dedicated lint script/config is defined (no Makefile target, no golangci config found).
- Use Go-native checks: `gofmt`-compatible code, clean build, and passing tests.
- If you changed imports/formatting, run formatter before finalizing.

### Release context (only when requested)
- Tag pattern `v*` triggers release workflow.
- Workflow uses: `goreleaser release --clean`.
- Do not run/publish release flows unless explicitly asked.

## Code Style and Conventions
These rules are based on existing code in `internal/cli`.

### Imports
- Keep imports Go-idiomatic and formatter-normalized.
- Use clean import blocks; remove unused imports immediately.
- Maintain readable grouping between stdlib and non-stdlib packages.

### Formatting
- Follow default Go formatting (`gofmt` style, tabs, spacing, import order).
- Keep functions focused; move helper logic into small internal functions.
- Avoid deep nesting when a guard clause can exit early.

### Types and state modeling
- Prefer explicit structs for state (`config`, `runtimeState`, `providedFlags`).
- Use concrete types (`int`, `[]string`, `bool`) rather than loose abstractions.
- Do not introduce `any`/empty interface unless an API requires it.

### Naming
- Use camelCase for unexported identifiers.
- Use action-oriented function names (`runApplyFlow`, `validateExecutionConfig`).
- Keep command vocabulary consistent: `setup`, `plan`, `preview`, `apply`, `doctor`.
- Use constants for command names, defaults, and enum-like values.

### Error handling
- Return errors; never silently ignore failures.
- Wrap propagated errors with context using `%w`.
- Use `errors.Is` for sentinel comparisons (e.g., user cancellation flows).
- Keep error messages explicit and operator-friendly.

### CLI output
- Preserve human-readable CLI output style.
- Keep sectioned summaries consistent (`printSection`, run summaries, doctor output).
- Do not print secrets or plaintext credentials in logs/summaries.
- Keep password displays redacted (`[hidden password]` pattern).

### Filesystem and permissions
- Follow existing permission model:
  - directories: `0o755`
  - sensitive files (inventory/auth/metadata): `0o600`
  - non-sensitive generated docs/vars: `0o644` where current code uses it
- Do not relax permissions for secret-bearing files.

### Tests
- Put tests in `*_test.go` and keep them deterministic.
- Use `t.TempDir()` for filesystem interactions.
- Use precise assertions with clear `t.Fatalf` messages.
- Avoid tests requiring external servers/services.
- When behavior changes, update tests in the same change.

## Change Checklist for Commands/Flags
If you add or modify a command/flag, update all applicable areas:

1. Argument parsing and defaults in `internal/cli/app.go`.
2. Validation and runtime behavior in `internal/cli/app.go` / `internal/cli/runtime.go`.
3. Help output (`printUsage`, `printCommandUsage`).
4. Shell completion logic in `internal/cli/completion.go`.
5. Interactive prompt behavior in `internal/cli/interactive.go` (if relevant).
6. Tests in `internal/cli/app_test.go`.
7. User docs in `README.md` and `docs/`.

## Minimum Verification Before Finishing
For code changes, run at least:

```bash
go test ./...
go build -o bin/civa .
```

For docs-only changes, ensure commands/paths still match actual code.

## Safety Boundaries (Merged from AGENT.md)
- Do not remove or restructure `ansible/collections` unless the task explicitly requires it and verification is included.
- Do not change release automation files (`.goreleaser.yaml`, `.github/workflows/release.yml`) unless the request is release-related.
- Avoid destructive edits to user runtime artifacts under `~/.civa/runs`; use official commands such as `plan remove`.
- Never expose credentials in output, plans, logs, or newly added docs.

## Source of Truth
- `AGENTS.md` is the canonical agent guidance file for this repository.
