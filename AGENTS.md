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

## Configuration & Agent Instructions

Start from `config/config.example.yaml`; keep credentials and runtime data untracked. Preserve unrelated working-tree changes.

Before requirements, scheduling, or development, read `.codex/MEMORY.md`. Follow its linked 20-week baseline and latest user-approved revisions; synchronize approved plan changes. Planned work is not completed work or authorization to deploy, publish, or send messages.
