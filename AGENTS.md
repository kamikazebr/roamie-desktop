# Repository Guidelines

## Project Structure & Module Organization
- `cmd/server` exposes the HTTP/VPN server entry point and pulls concrete handlers from `internal/server`.
- `cmd/client` owns the CLI surface, delegating to `internal/client` and shared DTOs/utilities in `pkg/models` and `pkg/utils`.
- Keep domain logic inside `internal/*`; only promote reusable, dependency-free code into `pkg`.
- `scripts/` bundles build, migration, and smoke-test helpers; run them from the repo root.
- `config/firebase-service-account.json` is a placeholder—keep real credentials and WireGuard artifacts outside git.

## Build, Test, and Development Commands
- `./scripts/docker-dev.sh setup` spins up the local PostgreSQL stack; rerun after container resets.
- `./scripts/build.sh` compiles `roamie-server` and `roamie` with Go 1.23.
- `go test ./...` runs unit tests across server and client packages.
- `./scripts/test-device-registration.sh` and `./scripts/test-device-auth.sh` deliver interactive smoke tests against a running WireGuard-enabled server.

## Coding Style & Naming Conventions
- Enforce `gofmt -w` and `goimports`; keep tabs for indentation and group stdlib/internal/third-party imports.
- Package names stay short, lowercase, and no underscores; exported symbols use PascalCase, while CLI commands remain dashed (e.g., `device disconnect`).
- Favor helpers in `pkg/utils` over ad-hoc copies; avoid cycles by sharing contracts via `pkg/models`.
- Store configuration templates in `configs/`; environment variables use uppercase snake case (`RESEND_API_KEY`, `DATABASE_URL`).

## Testing Guidelines
- Place tests beside code as `*_test.go`, naming functions `TestComponentScenario`; use table-driven patterns for handler logic.
- Run `go test ./...` before pushing and cover critical paths like device lifecycle and biometric auth flows.
- Keep unit tests hermetic by mocking Resend and WireGuard integrations via `pkg/utils`; reserve shell scripts for end-to-end validation.

## Commit & Pull Request Guidelines
- Prefer Conventional Commit prefixes (`feat:`, `fix:`, `chore:`) with optional scopes (`feat(server): enforce device quota`).
- Keep commits atomic and include schema/config updates alongside dependent code.
- Pull requests should capture behavior changes, required deployment or migration steps, and testing evidence (command output, script name, screenshots).
- Reference related issues and call out security or rollback considerations when editing infrastructure scripts.

## Security & Configuration Tips
- Never commit live secrets: `.env`, `config/firebase-service-account.json`, and WireGuard keys stay in local or deployment-specific stores.
- When editing `scripts/setup-server.sh` or firewall rules, document required sudo privileges and verify `scripts/restore-wireguard.sh` still resets state cleanly.
- Regenerate QR codes and device configs whenever authentication flows change; record the action in PR notes.
