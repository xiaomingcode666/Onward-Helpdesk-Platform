# Repository Guidelines

## Project Structure & Module Organization

- `cmd/server/` starts the Go API. `internal/handlers/`, `services/`, `repositories/`, and `models/` separate HTTP handling, business logic, persistence, and data models. `internal/ai/` contains AI workflows and retrieval.
- `web/` contains Next.js App Router pages (`app/`), shared components (`components/`, `packages/railops-ui/`), utilities (`lib/`), translations (`messages/`), and assets (`public/`).
- Go tests live beside source files; browser tests live in `web/e2e/`. Configuration examples, deployment resources, and utilities live in `config/`, `deploy/`, and `scripts/`.

## Build, Test, and Development Commands

Match CI: Go 1.26, Node.js 22, and pnpm 10.30.2. Run these from the repository root:

- `go mod download` and `pnpm --dir web install --frozen-lockfile`: install dependencies.
- `go run ./cmd/server -config config/config.yaml`: start the backend with local configuration.
- `pnpm --dir web dev`: start the frontend on port 3000 using the established Turbopack configuration.
- `pnpm --dir web build:sdk`, then `pnpm --dir web build`: build the SDK and dynamic dashboard.
- `make build`: build the static frontend and bundled backend into `dist/`; requires Make and a Bash-compatible environment.
- `go vet ./...`, `pnpm --dir web lint`, and `pnpm --dir web typecheck`: run static checks.

## Coding Style & Naming Conventions

Format Go with `gofmt` (tabs); use lowercase package names, `snake_case.go` filenames, and PascalCase exported identifiers. Follow adjacent TypeScript formatting, normally two spaces; use PascalCase components and camelCase functions. ESLint uses Next.js Core Web Vitals and TypeScript rules. Keep business logic in services and tenant checks explicit.

## Testing Guidelines

Use Go's `testing` package and Testify with `*_test.go` files and `TestXxx` functions. Run `go test ./...`; CI additionally uses `-race -coverprofile=coverage.out -covermode=atomic`. Coverage is collected without a numeric minimum.

Playwright tests use `*.spec.ts`; register new suites in `web/playwright.config.ts`. Run `pnpm --dir web test:e2e:p0` against an isolated fixture environment; follow `.github/workflows/ci.yml` for services, seeding, and credentials. Cover changed behavior, permissions, and tenant isolation.

## Commit & Pull Request Guidelines

History is limited; follow its `chore: import Onward Helpdesk platform` style with concise `type: summary` messages. PRs should explain behavior changes, link relevant issues or requirement IDs, list executed checks, and include screenshots for UI changes.

## Mandatory Local Verification Before Push

The user requires all tests to pass locally before any code is pushed to a remote repository. Aim for the first remote CI run to pass; never use repeated pushes as a substitute for local debugging.

- Validate the exact final commit intended for push in a clean, isolated checkout. Preserve concurrent development and unrelated changes; keep build caches and temporary Go sources outside that checkout.
- Match `.github/workflows/ci.yml`: Linux (local Docker or WSL when working on Windows), UTC, Go/Node/pnpm versions, locked dependencies, and isolated PostgreSQL, Redis, Qdrant and acceptance fixtures. Windows-only results are insufficient for Linux CI compatibility.
- Run the complete repository test suites and every CI check locally: all backend tests with race detection and coverage (all groups in `scripts/ci-backend-tests.sh`, including `core`), formatting, Vet, Go vulnerability scanning, frontend typecheck/Lint/SDK and application builds, dependency audit, P0 browser acceptance, and committed-history secret scanning. Required checks must not be replaced with targeted tests alone.
- A failed, skipped, interrupted, unavailable, or still-running required check blocks pushing. Fix local environment limitations or report the blocker; do not push just to discover whether CI passes. Do not disable tests, weaken assertions, bypass checks, or expand exclusions to obtain a green result.
- Record the tested commit, commands, environment and results. Subsequent code, dependency or configuration changes invalidate the previous final verification; complete verification again before pushing the changed commit.
- Once the final local checks all pass, push the verified commit together. Monitor remote CI and merge only after all remote checks also pass. If remote CI unexpectedly fails, reproduce and fix the cause locally and repeat the complete verification before another push.

## Configuration & Agent Instructions

Start from `config/config.example.yaml`; keep credentials and runtime data untracked. Preserve unrelated working-tree changes.

The user-designated requirements source is [the live Chapter 4 requirements and gap table](https://www.kdocs.cn/l/cb6HdI2uBMOG), bound on 2026-09-14. Read `work/需求开发依据.md` and, when available, `.codex/MEMORY.md` before requirements, scheduling, or development. Check each requirement ID against the live table and current code; follow the user's latest confirmed scope and priorities. Local spreadsheets and the 20-week baseline are historical references, not overrides of the live source or newer user decisions. Record access limitations and conflicts instead of guessing missing content or silently changing delivery status. Planned work is not completed work or authorization to deploy, publish, or send messages.

The user authorizes updating the corresponding completion status in that online table after each requirement is implemented and verified. Follow the status rules in `work/需求开发依据.md`; distinguish development completion from acceptance, preserve unrelated cells, and verify the saved value. If online editing is unavailable, record the pending update locally and report it without claiming the online table was updated.
