# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go 1.22 microservices monorepo managed with [`go.work`](/Users/yuki-munakata/Documents/hobby/microservice/go.work). Core services live in `services/` (`gateway`, `user`, `product` currently wired into the workspace). Each service follows `cmd/` for entrypoints, `internal/` for application code, and `migrations/` for schema changes. Shared infrastructure code lives in `pkg/` (`grpcserver`, `health`, `logger`, `tracer`, `errors`). API contracts live in `proto/`, deployment manifests in `deployments/`, automation scripts in `scripts/`, and design references in `docs/`.

## Build, Test, and Development Commands
Use the [`Makefile`](/Users/yuki-munakata/Documents/hobby/microservice/Makefile) as the primary entry point.

- `make setup`: verify and install local tooling such as `golangci-lint`.
- `make dev-infra`: start PostgreSQL, Redis, Kafka, and Elasticsearch for local work.
- `make dev`: start the full Compose stack.
- `make proto`: regenerate Go code from `.proto` files.
- `make migrate-up` or `make migrate-up-user`: apply database migrations.
- `make test`, `make test-user`, `make test-pkg`: run service or shared-package tests with `-race -count=1`.
- `make lint`, `make lint-pkg`: run `golangci-lint`.
- `make build` or `make build-user`: build Docker images for deployment.

## Coding Style & Naming Conventions
Follow standard Go conventions: format with `gofmt`, keep imports clean, and use tabs as emitted by Go tooling. Package names are short and lowercase; exported identifiers use `CamelCase`, unexported helpers use `camelCase`, and tests live in `*_test.go`. Keep service internals layered as `repository -> service -> handler`; place cross-cutting logic in `pkg/` instead of duplicating it across services.

## Testing Guidelines
Tests use Go’s `testing` package plus `stretchr/testify`. Prefer table-driven tests where the scenarios are repetitive, and keep mocks local to the test package when possible. Run `make test` before opening a PR and generate coverage with `make test-coverage` when touching critical flows. The project docs target 80%+ coverage for the full service set.

## Commit & Pull Request Guidelines
Git history currently uses Conventional Commit style (`feat: initial commit`), so continue with prefixes like `feat:`, `fix:`, and `chore:`. Keep commits scoped to one service or shared concern. PRs should include a short summary, affected services, migration or proto notes if applicable, and proof of validation (`make test`, `make lint`). Include API examples or screenshots only when a gateway response or operational workflow changes.

## Security & Configuration Tips
Do not commit secrets; keep runtime configuration in environment variables and Kubernetes/Docker overrides. When changing contracts, update both `proto/` or OpenAPI specs and the consuming service in the same branch.
