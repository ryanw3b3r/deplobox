# Repository Guidelines

## Project Structure & Module Organization

- `cmd/deplobox/`: CLI entrypoint wiring HTTP server, config, and logging.
- `internal/`: core packages
  - `server/`: routing, middleware, signature verification, handlers.
  - `deployment/`: executor, locking, post-deploy steps.
  - `history/`: SQLite persistence.
  - `project/`: config loading/validation.
- `stubs/`: systemd service template.
- `tests/`: reserved for integration harnesses; unit tests live beside code as `*_test.go`.

## Build, Test, and Development Commands

- `make build` — compile `deplobox` (stripped) to repo root.
- `make run` — run with `projects.yaml`; override via flags/env vars.
- `make test` — run all Go tests with coverage summary.
- `make test-coverage` — emit `coverage.out` and `coverage.html`.
- `make cross-compile` — build Linux AMD64/ARM64 binaries.
- `make lint` — `go vet ./...` static checks.
- `make fmt` — `gofmt -s -w .` formatting.
- `make deps` — `go mod download` + `go mod tidy`.
- Targeted run example: `go test ./internal/server -run TestSignature`.

## Coding Style & Naming Conventions

- Use `gofmt`; tab indent and default import ordering are required.
- Exported identifiers: PascalCase; unexported: camelCase; package names stay short (`server`, `project`).
- Prefer small interfaces and constructors (`New...`); keep files focused.
- Wrap errors with context (`fmt.Errorf("...: %w", err)`) and return early.
- Config keys remain snake_case to mirror YAML.

## Testing Guidelines

- Keep tests next to code (`*_test.go`); favor table-driven cases and helper builders.
- Handler tests should set explicit bodies/headers to stay deterministic.
- Coverage artifacts (`coverage.out`, `coverage.html`) are local-only.
- Add integration smoke tests under `tests/` when adding new external interactions (git, SQLite, shell commands).

## Commit & Pull Request Guidelines

- Commit messages: imperative line ≤72 chars (e.g., `Add lock around deployment runner`); expand in body when behavior changes.
- Scope commits narrowly; avoid mixing refactors with fixes/features.
- PRs should note behavior change, test evidence (`make test`), risk/rollback, and linked issue. Screenshots/log snippets only when user-visible output changes.

## Security & Configuration Tips

- Never commit real secrets or `projects.yaml`; use redacted samples.
- Use absolute, validated deployment paths; restrict `post_deploy` to predefined commands.
- Default `DEPLOBOX_EXPOSE_OUTPUT=false` in production; enable only for debugging.
