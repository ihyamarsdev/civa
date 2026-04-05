# AGENTS.md

Guidance for coding agents working in `civa`.

## Project Snapshot
- Language: Go (`go 1.26`, module `civa`).
- Type: interactive CLI for VPS automation using embedded Ansible assets.
- Entry point: `main.go` -> `internal/cli.Run(...)` in `internal/cli/run.go`.
- CLI wiring: `internal/cli/cmd/root.go` (Cobra commands and flags).
- Request contract: `internal/cli/domain/request.go`.
- Runtime execution: `internal/cli/infra/*` (`adapter.go`, `runner.go`, `cloudflare.go`, `interactive.go`, `completion.go`, `cli.go`).
- Embedded assets: `ansible/` materialized at runtime.
- Release pipeline: `.goreleaser.yaml` + `.github/workflows/release.yml`.

## Operational Flow
- Primary flow: collect operator input -> generate run artifacts (`~/.civa/runs/<run-id>/`) -> review plan -> apply plan.
- This repository focuses on CLI orchestration and execution planning; provisioning logic is delegated to embedded Ansible assets.
- `civa setup` is the bootstrap command for first-time SSH key installation on a target host.
- `civa plan init` assumes key-based SSH access (password mode is not supported for planning).
- `plan review`/`plan edit` and `apply` operate on an existing generated plan (by plan name or `--plan-file`).
- Cloudflare flow now uses auth profiles first (`civa auth cloudflare ...`), and tools read tokens from those profiles (`civa tools cloudflare zones ... --profile <name>`).

## Rule Sources (Cursor/Copilot)
- `.cursor/rules/`: not found.
- `.cursorrules`: not found.
- `.github/copilot-instructions.md`: not found.
- Use this file and existing repository patterns as the authoritative guide.

## Repo Areas You Will Touch Most
- `main.go`: binary entrypoint.
- `internal/cli/run.go`: argument normalization and dependency wiring (`infra.NewLegacyRunner` -> `app.NewService` -> `cmd.NewRoot`).
- `internal/cli/cmd/root.go`: Cobra command tree, flags, and request mapping for commands including `auth` and `tools`.
- `internal/cli/domain/request.go`: command/action constants and request payload fields.
- `internal/cli/app/service.go`: thin app service that delegates requests to the runner.
- `internal/cli/infra/adapter.go`: converts `domain.Request` into runtime config and dispatches command flows.
- `internal/cli/infra/runner.go`: core runtime operations (plan/apply/setup/config/secret/doctor and shared storage helpers).
- `internal/cli/infra/cloudflare.go`: Cloudflare auth profile CRUD + tools/zones CRUD + API client.
- `internal/cli/infra/interactive.go`: interactive prompt flows (huh-based).
- `internal/cli/infra/completion.go`: shell completion generation and hidden completion routing.
- `internal/cli/infra/cli.go`: command usage/help output and many shared CLI helpers.
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
go test ./internal/cli/cmd
go test ./internal/cli/infra
```

### Run a single test function (important)
```bash
go test ./internal/cli/cmd -run '^TestRootRunRoutesToolsCloudflareZonesList$'
```

### Run a single subtest
```bash
go test ./internal/cli/infra -run 'TestName/Subcase'
```

### Re-run without cache
```bash
go test ./internal/cli/infra -run '^TestName$' -count=1
```

### Verbose single-test output
```bash
go test ./internal/cli/infra -run '^TestName$' -v
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
- Prefer explicit structs for state (`config`, `domain.Request`, `providedFlags`).
- Use concrete types (`int`, `[]string`, `bool`) rather than loose abstractions.
- Do not introduce `any`/empty interface unless an API requires it.

### Naming
- Use camelCase for unexported identifiers.
- Use action-oriented function names (`runApplyFlow`, `validateExecutionConfig`, `runCloudflareZonesListFlow`).
- Keep command vocabulary consistent: `setup`, `plan`, `apply`, `doctor`, `auth`, `tools`, `secret`, `config`.
- Use constants for command names, defaults, and enum-like values.

### Error handling
- Return errors; never silently ignore failures.
- Wrap propagated errors with context using `%w`.
- Use `errors.Is` for sentinel comparisons (e.g., user cancellation or missing secrets).
- Keep error messages explicit and operator-friendly.

### CLI output
- Preserve human-readable CLI output style.
- Keep sectioned summaries consistent (`printSection`, run summaries, doctor output).
- Do not print secrets or plaintext credentials in logs/summaries.
- Keep password displays redacted (`[hidden password]` pattern).

### Interactive forms
- Use `github.com/charmbracelet/huh` for interactive form/prompt flows.
- Avoid ad-hoc stdin reads for form input; prefer `huh` input/select/confirm components.

### Filesystem and permissions
- Follow existing permission model:
  - directories: `0o755` (`~/.civa/secrets` directory is enforced as `0o700`)
  - sensitive files (inventory/auth/metadata/secrets): `0o600`
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

1. Cobra command wiring and flag parsing in `internal/cli/cmd/root.go`.
2. Command/action constants and request fields in `internal/cli/domain/request.go`.
3. Runtime request mapping in `internal/cli/infra/adapter.go`.
4. Runtime command behavior in `internal/cli/infra/runner.go` and provider-specific files (for Cloudflare: `internal/cli/infra/cloudflare.go`).
5. Help output in `internal/cli/infra/cli.go` (`printUsage`, `printCommandUsage`).
6. Shell completion logic in `internal/cli/infra/completion.go`.
7. Interactive prompt behavior in `internal/cli/infra/interactive.go` (if relevant).
8. Tests in `internal/cli/cmd/root_test.go`, `internal/cli/infra/*_test.go`, and `internal/cli/run_test.go` as applicable.
9. User docs in `README.md` and `docs/`.

## Minimum Verification Before Finishing
For code changes, run at least:

```bash
go test ./...
go build -o bin/civa .
```

For docs-only changes, ensure commands/paths still match actual code.

## Safety Boundaries
- Do not remove or restructure `ansible/collections` unless the task explicitly requires it and verification is included.
- Do not change release automation files (`.goreleaser.yaml`, `.github/workflows/release.yml`) unless the request is release-related.
- Avoid destructive edits to user runtime artifacts under `~/.civa/runs`; use official commands such as `plan remove`.
- Never expose credentials in output, plans, logs, or newly added docs.

## Source of Truth
- `AGENTS.md` is the canonical agent guidance file for this repository.
