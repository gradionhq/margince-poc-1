# Margince WP0 gates. See docs/plans/2026-06-24-wp0-repo-foundation-setup.md
# GNU make 3.81 compatible (recipes use real tabs).

GO_DIRS  := backend crm-de cli/crm-gen cli/craft
SEED_DIR := backend/seed
# Use host psql when available; otherwise fall back to psql inside the infra-postgres container.
PSQL     := $(if $(shell command -v psql),PGPASSWORD=margince psql -h localhost -U margince -d margince,docker exec -i -e PGPASSWORD=margince infra-postgres-1 psql -U margince -d margince)
# Same fallback, but against the postgres maintenance DB — needed for CREATE/DROP DATABASE.
PSQL_ADMIN := $(if $(shell command -v psql),PGPASSWORD=margince psql -h localhost -U margince -d postgres,docker exec -i -e PGPASSWORD=margince infra-postgres-1 psql -U margince -d postgres)
GOFILES := $(shell find backend crm-de cli -name "*.go" 2>/dev/null)
GO_COVER_MIN := 0
DOCKER_COMPOSE := $(if $(shell docker compose version >/dev/null 2>&1 && echo yes),docker compose,docker-compose)
COMPOSE := $(DOCKER_COMPOSE) -f infra/docker-compose.dev.yml
DATABASE_URL ?= postgres://margince:margince@localhost:5432/margince?sslmode=disable
TEST_DATABASE_URL ?= postgres://margince:margince@localhost:5432/margince_test?sslmode=disable
REDIS_URL ?= redis://localhost:6379
BLOBSTORE_ENDPOINT   ?= localhost:9000
BLOBSTORE_BUCKET     ?= transcripts
BLOBSTORE_ACCESS_KEY ?= minioadmin
BLOBSTORE_SECRET_KEY ?= minioadmin
MIGRATE := $(if $(shell command -v migrate),migrate,$(HOME)/go/bin/migrate)
export PATH := $(HOME)/go/bin:$(PATH)
export DATABASE_URL
export TEST_DATABASE_URL
# Integration tests assume every backing service is provisioned (Postgres + Redis +
# MinIO) and never skip on a missing env var — so the runner must export them all.
export REDIS_URL
export BLOBSTORE_ENDPOINT
export BLOBSTORE_BUCKET
export BLOBSTORE_ACCESS_KEY
export BLOBSTORE_SECRET_KEY

GOLANGCI := $(if $(shell command -v golangci-lint),golangci-lint,$(HOME)/go/bin/golangci-lint)
GOFUMPT  := $(if $(shell command -v gofumpt),gofumpt,$(HOME)/go/bin/gofumpt)
GOLANGCI_CONFIG := $(CURDIR)/.golangci.yml
# golangci-lint's own result cache is content-addressed but records absolute
# source paths. Sharing the default machine-wide cache across git worktrees with
# byte-identical files makes lint read stale/wrong paths from whichever worktree
# populated the cache first (e.g. gosec "no such file or directory"). Scoping the
# cache inside $(CURDIR) gives every worktree — and the main checkout — its own.
export GOLANGCI_LINT_CACHE := $(CURDIR)/.tmp/golangci-cache

.PHONY: help check check-backend check-q check-go check-fe check-fe-static check-craft-doc check-doc-style craft fmt fmt-check vet lint go-file-length test test-v test-cover test-cover-check test-integration test-it test-integration-serial test-liveuat test-lanes arch-lint fitness-jurisdiction audit-coverage audit-coherence rls-store-path contract-lint gen-types gen-types-check contract-breaking-check gen-field gen-manifests gen-manifests-check tools tools-go tidy build run dev psql clean install fe-install fe-build fe-preview fe-lint fe-typecheck fe-format fe-dev storybook fe-test test-contracts infra-up infra-down infra-reset infra-logs db-wait migrate-up migrate-down migrate-status migrate-create test-db-up test-db-reset seed seed-dev seed-reset ds-purity font-lock icon-lint check-image-pins uat_env uat_env_stop fe-uat

help: ## Show targets
	@grep -hE "^[a-zA-Z_-]+:.*## " $(MAKEFILE_LIST) | awk "BEGIN{FS=\":.*## \"}{printf \"  %-20s %s\\n\",\$$1,\$$2}"

# --- Aggregate gates ---
# ONE ordered gate. Cheapest static checks first so it fails fast; the test suites
# run LAST. Order: format → lint → file-length → codegen-drift → DAG/invariants →
# frontend static → Go tests → frontend tests. `make check-q` is the quiet wrapper.
check: check-backend check-fe ## The gate: backend (format→lint→invariants→Go tests) then frontend (static + tests)
	@echo "OK: make check passed"
# Backend half of the gate — everything except the FE static+tests. CI's
# backend-gates job runs exactly this, keeping CI one-to-one with local `make check`.
check-backend: fmt-check vet lint go-file-length \
       gen-types-check contract-breaking-check gen-manifests-check arch-lint fitness-jurisdiction \
       audit-coverage audit-coherence rls-store-path check-craft-doc check-doc-style check-image-pins test-lanes \
       test ## Backend gate: format → lint → codegen-drift → DAG/invariants → Go unit tests
	@echo "OK: make check-backend passed"
check-image-pins: ## Fail if any .github/workflows/*.yml uses: line has a floating tag
	@bash scripts/check-image-pins.sh
check-craft-doc: ## Assert the ## Craftsmanship section is present in AGENTS.md
	@grep -q "^## Craftsmanship" AGENTS.md || { echo "FAIL: AGENTS.md is missing the '## Craftsmanship' section (docs/quality/craftsmanship.md)"; exit 1; }
	@echo "OK: AGENTS.md ## Craftsmanship present"
check-doc-style: ## Assert subsystem docs are system explanations, not code walkthroughs (AGENTS.md domain-doc style)
	@bash scripts/check-subsystem-doc-style.sh
check-q: ## Quiet check; full log in .tmp/check.log, excerpt on failure
	@mkdir -p .tmp
	@if $(MAKE) check > .tmp/check.log 2>&1; then echo "OK: check-q passed"; else echo "FAIL: check-q (last 40 lines):"; tail -n 40 .tmp/check.log; exit 1; fi
# Standalone convenience gates (the inner-loop subsets `check` is composed from).
check-go: fmt-check vet lint go-file-length test ## Go gate: gofumpt + vet + golangci-lint (T1, §3) + file-length (§3.2) + tests
check-fe-static: fe-lint fe-typecheck ds-purity font-lock icon-lint ## Frontend static: biome + tsc + design-system/font/icon purity (no tests)
check-fe: check-fe-static fe-test ## Frontend gate: static checks + tests

# --- Go ---
fmt: ## Format Go files (gofumpt — stricter-than-gofmt, §3.1)
	@$(GOFUMPT) -w $(GOFILES)
fmt-check: ## Fail if any Go file is not gofumpt-formatted
	@out=`$(GOFUMPT) -l $(GOFILES)`; if [ -n "$$out" ]; then echo "not gofumpt-formatted:"; echo "$$out"; exit 1; fi
vet: ## go vet across all modules
	@for d in $(GO_DIRS); do echo "vet $$d"; (cd $$d && go vet ./...) || exit 1; done
lint: ## golangci-lint — the T1 ruleset (architecture/18 §3); runs the committed .golangci.yml
	@command -v $(GOLANGCI) >/dev/null 2>&1 || [ -x "$(GOLANGCI)" ] || { echo "FAIL: golangci-lint not installed — run 'make tools'"; exit 1; }
	@for d in $(GO_DIRS); do echo "lint $$d"; (cd $$d && $(GOLANGCI) run --config $(GOLANGCI_CONFIG) ./...) || exit 1; done
go-file-length: ## Fail on any Go file over the §3.2 LOC cap (god-file guard)
	@bash scripts/check-go-file-length.sh
test: ## go test across all modules
	@for d in $(GO_DIRS); do echo "test $$d"; (cd $$d && go test ./...) || exit 1; done
test-v: ## go test -v
	@for d in $(GO_DIRS); do (cd $$d && go test -v ./...) || exit 1; done
test-cover: ## Go tests with coverage
	@for d in $(GO_DIRS); do (cd $$d && go test -cover ./...) || exit 1; done
test-cover-check: ## Enforce GO_COVER_MIN (raise as code grows)
	@echo "coverage threshold: $(GO_COVER_MIN)% (WP0 placeholder)"; $(MAKE) test-cover
test-integration: test-db-up ## Go integration tests (needs make infra-up). Parallel, fully provisioned, zero-skip.
	# Each //go:build integration package runs concurrently on its own throwaway state:
	# a margince_test clone (CREATE DATABASE ... TEMPLATE), a private Redis logical db, and
	# a private MinIO bucket — so packages share nothing. Within a package: still -p 1 (the
	# green-today sequential model), so no test file changed. Same zero-skip teeth as before
	# (a skipped test fails the run). Tune concurrency with INTEGRATION_JOBS=N.
	@bash scripts/test-integration-parallel.sh

test-it: test-db-up ## Run ONE integration package/test on a throwaway clone: make test-it DIR=backend/internal/modules/directory [RUN=TestName]
	@bash scripts/test-integration-one.sh "$(DIR)" "$(RUN)"

test-integration-serial: test-db-up ## Escape hatch: the old sequential lane, all packages on the shared margince_test DB (debugging only).
	# -p 1 across one shared DB. Slower; kept for diagnosing a parallel-isolation issue.
	@log=`mktemp`; tmp=`mktemp`; \
	for d in $(GO_DIRS); do echo "integration $$d"; \
	  (cd $$d && go test -p 1 -tags=integration -v -count=1 -timeout=300s ./...) > $$tmp 2>&1; st=$$?; \
	  cat $$tmp; cat $$tmp >> $$log; \
	  if [ $$st -ne 0 ]; then rm -f $$tmp $$log; exit 1; fi; \
	done; \
	rm -f $$tmp; \
	if grep -q -- '--- SKIP' $$log; then \
	  echo "FAIL: integration tests must not skip — provision the env/service, do not skip:"; \
	  grep -- '--- SKIP' $$log; rm -f $$log; exit 1; \
	fi; \
	rm -f $$log; echo "OK: integration passed with 0 skips"

test-liveuat: ## Live-stack UAT Go harnesses (//go:build liveuat). Needs a migrated+seeded dev stack: make infra-up migrate-up seed-reset run
	@cd backend && DATABASE_URL="$(DATABASE_URL)" go test -tags=liveuat -count=1 -timeout=300s ./...

test-lanes: ## Enforce test-lane separation: no untagged (unit) test may open a real DB/Redis (those belong in the integration lane)
	@bash scripts/check-test-lanes.sh

test-db-up: db-wait ## Create margince_test DB + run migrations (idempotent)
	@echo "Creating margince_test database (idempotent)..."
	@$(PSQL_ADMIN) -c "CREATE DATABASE margince_test" 2>/dev/null || true
	@$(MIGRATE) -path backend/migrations -database "$(TEST_DATABASE_URL)" up
	@echo "test-db-up: margince_test ready"

seed: ## Run a seed SQL file: make seed FILE=dev.sql
	@$(PSQL) < $(SEED_DIR)/$(FILE) && echo "seed: $(FILE) applied"
seed-dev: ## Apply the default dev seed (backend/seed/dev.sql) — idempotent
	@$(MAKE) seed FILE=dev.sql
seed-reset: ## Drop ALL dev-workspace data then re-seed (destructive for the dev workspace)
	@$(PSQL) -v ON_ERROR_STOP=1 < $(SEED_DIR)/reset.sql && $(MAKE) seed-dev

test-db-reset: ## Drop + recreate margince_test (destructive)
	@$(PSQL_ADMIN) -c "DROP DATABASE IF EXISTS margince_test"
	@$(MAKE) test-db-up
tidy: ## Sync workspace + module deps
	@go work sync; for d in $(GO_DIRS); do (cd $$d && go mod tidy) || true; done
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell git log -1 --format=%cI 2>/dev/null || echo "1970-01-01T00:00:00Z")
LDFLAGS    := -X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -s -w

build: ## Build the server binary (reproducible: -trimpath -buildvcs=false + ldflags)
	@cd backend/cmd/api && go build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o ../../../bin/server . && echo "built bin/server"
run: ## Run the server
	@cd backend/cmd/api && BLOBSTORE_ENDPOINT=$(BLOBSTORE_ENDPOINT) BLOBSTORE_BUCKET=$(BLOBSTORE_BUCKET) BLOBSTORE_ACCESS_KEY=$(BLOBSTORE_ACCESS_KEY) BLOBSTORE_SECRET_KEY=$(BLOBSTORE_SECRET_KEY) go run .
dev: run ## Alias for run — start the server at :8080
psql: ## Open a psql shell against the dev database
	@$(PSQL)
clean: ## Remove build artifacts
	@rm -rf bin .tmp

# --- Invariant gates ---
arch-lint: ## Enforce the module DAG (go-arch-lint). A keystone invariant is never optional (architecture/18 §2.3).
	@command -v go-arch-lint >/dev/null 2>&1 || [ -x "$(HOME)/go/bin/go-arch-lint" ] || { echo "FAIL: go-arch-lint not installed — run 'make tools'"; exit 1; }
	@cd backend && $(if $(shell command -v go-arch-lint),go-arch-lint,$(HOME)/go/bin/go-arch-lint) check
fitness-jurisdiction: ## No country-specific strings in core (ADR-0042)
	@bash scripts/check-no-jurisdiction.sh
audit-coverage: ## Fail if any SoR mutation bypasses the crm-audit seam (B-EP07.3)
	@bash scripts/check-audit-coverage.sh .
audit-coherence: ## Fail if audit_log action/actor_type CHECK drifts from the contract (crm.yaml)
	@bash scripts/check-audit-action-coherence.sh .
rls-store-path: ## crm-core stores must engage RLS via withWorkspaceTx, not the bare superuser pool
	@bash scripts/check-rls-store-path.sh

# --- Contract codegen (crm.yaml -> Go + TS) ---
contract-lint: ## Fast pre-flight: every local $$ref in crm.yaml resolves (catches typo'd schema names)
	@if [ -d node_modules ]; then node scripts/contract-lint.mjs backend/api/crm.yaml; else echo "skip contract-lint: run make fe-install first"; fi
gen-types: ## Generate Go + TS contract types from the in-repo backend/api/crm.yaml
	@bash scripts/gen-types.sh write
gen-types-check: ## Fail if generated contract types drift from crm.yaml
	@bash scripts/gen-types.sh check
contract-breaking-check: ## Fail on breaking crm.yaml changes since origin/main (oasdiff severity→policy: ERR blocks, WARN/INFO pass)
	@bash scripts/check-contract-breaking.sh
gen-field: ## Scaffold a field: make gen-field ARGS="person nickname text string"
	@cd cli/crm-gen && go build -o ../../bin/crm-gen .
	@./bin/crm-gen field $(ARGS)
gen-manifests: ## Regenerate backend/cmd/api/imports_gen.go from self-registered entries
	@cd cli/crm-gen && go build -o ../../bin/crm-gen . && cd ../..
	@./bin/crm-gen manifests
gen-manifests-check: ## Fail if backend/cmd/api/imports_gen.go drifts from connectors/workflows/tools dirs
	@$(MAKE) gen-manifests
	@git diff --exit-code backend/cmd/api/imports_gen.go

craft: ## Build the craftsmanship gate CLI (review / annotate / residue / eval / dispute)
	@cd cli/craft && go build -o ../../bin/craft . && echo "built bin/craft"

# --- Tooling bootstrap ---
tools: tools-go ## Install codegen + lint binaries + Playwright browser (full machine bootstrap)
	@cd frontend && pnpm exec playwright install chromium && echo "Playwright Chromium installed"
tools-go: ## Install Go lint/codegen binaries (oapi-codegen, go-arch-lint, golangci-lint, gofumpt, oasdiff, migrate) — machine-wide ($(HOME)/go/bin), skips each binary already on PATH
	@command -v oapi-codegen >/dev/null 2>&1 || go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	@command -v go-arch-lint >/dev/null 2>&1 || go install github.com/fe3dback/go-arch-lint@latest
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@command -v gofumpt >/dev/null 2>&1 || go install mvdan.cc/gofumpt@latest
	@command -v oasdiff >/dev/null 2>&1 || go install github.com/oasdiff/oasdiff@latest
	@command -v migrate >/dev/null 2>&1 || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Go lint/codegen tools ready in $(HOME)/go/bin"

# --- Frontend (skip cleanly if deps not installed) ---
install: fe-install tools-go ## One-shot fresh-worktree setup (called by `swarm worktree-init`): frontend deps + lint/codegen tools; extend here as new setup steps are needed
fe-install: ## Install frontend deps
	@pnpm install
fe-build: ## Production build of the web app
	@pnpm --filter @gradion/crm-web run build
fe-preview: ## Preview the production build
	@pnpm --filter @gradion/crm-web run preview
fe-lint: ## Biome lint (skips if node_modules absent)
	@if [ -d node_modules ]; then pnpm exec biome check . ; else echo "skip fe-lint: run make fe-install first"; fi
fe-typecheck: ## tsc typecheck (skips if node_modules absent)
	@if [ -d node_modules ]; then pnpm -r --if-present run typecheck ; else echo "skip fe-typecheck: run make fe-install first"; fi
fe-format: ## Biome format --write
	@if [ -d node_modules ]; then pnpm exec biome format --write . ; fi
fe-dev: ## Start Vite dev server (web client)
	@pnpm --filter @gradion/crm-web exec vite
storybook: ## Start Storybook dev server
	@pnpm --filter @gradion/crm-web exec storybook dev --port 6006
ui-shots: ## Capture each story to frontend/.shots/*.png for visual review (opt-in, not a gate). Optional: ID=<substr>
	@if [ -d node_modules ]; then node frontend/scripts/capture-stories.mjs $(ID) ; else echo "skip ui-shots: run make fe-install first"; fi
fe-test: ## Run frontend unit + storybook component tests
	@if [ -d node_modules ]; then \
	  pnpm --filter @gradion/crm-web exec vitest run --project unit && \
	  pnpm --filter @gradion/crm-web exec vitest run --project storybook ; \
	else echo "skip fe-test: run make fe-install first"; fi
test-contracts: ## Run TypeScript contract compliance tests
	@pnpm --filter @gradion/crm-web exec vitest run --project unit src/lib/api-client/contract.test.ts
ds-purity: ## Design-system token purity check (no raw hex/px, correct gf- prefixes)
	@if [ -d frontend/src ]; then bash scripts/check-ds-purity.sh ; else echo "skip ds-purity: frontend/src not found"; fi
font-lock: ## Font-family lock lint — exactly Outfit / DM Sans / JetBrains Mono
	@if [ -d frontend/src ]; then bash frontend/scripts/check-font-lock.sh ; else echo "skip font-lock: frontend/src not found"; fi
icon-lint: ## Icon-glyph lock lint — UI chrome icons are Lucide only (🟢/🟡 dots confined to AutonomyDot)
	@if [ -d frontend/src ]; then bash frontend/scripts/check-icon-glyph.sh ; else echo "skip icon-lint: frontend/src not found"; fi

# --- Infra ---
infra-up: ## Start Postgres(pgvector) + Redis
	@$(COMPOSE) up -d
infra-down: ## Stop infra
	@$(COMPOSE) down
infra-reset: ## Stop + wipe volumes
	@$(COMPOSE) down -v
infra-logs: ## Tail infra logs
	@$(COMPOSE) logs -f

# --- Per-worktree UAT env (B5): own db crm_uat_<slug> on the shared infra + derived ports ---
uat_env: ## Spin a per-worktree UAT env (mandatory UAT_SLUG=<slug>): own db + derived ports; logs+stop under .tmp/uat/<slug>/
	@bash scripts/uat-env.sh up "$(UAT_SLUG)"
uat_env_stop: ## Stop a UAT env: make uat_env_stop UAT_SLUG=<slug> [DROP=1 also drops the db]
	@bash scripts/uat-env.sh stop "$(UAT_SLUG)" $(if $(DROP),--drop,)
db-wait: ## Block until postgres answers pg_isready (up to 30 attempts, 2s apart) — guards migrate-up/test-db-up against the infra-up cold-start race
	@for i in $$(seq 1 30); do \
	  if $(COMPOSE) exec -T postgres pg_isready -U margince -h 127.0.0.1 -p 5432; then \
	    echo "postgres is ready"; \
	    exit 0; \
	  fi; \
	  echo "postgres not ready yet (attempt $$i/30), sleeping 2s"; \
	  sleep 2; \
	done; \
	echo "FAIL: postgres did not become ready after 30 attempts"; \
	exit 1

# --- Migrations (golang-migrate; schema from data-model.md) ---
migrate-up: db-wait ## Apply all pending migrations
	@$(MIGRATE) -path backend/migrations -database "$(DATABASE_URL)" up
migrate-down: ## Roll back the last migration
	@$(MIGRATE) -path backend/migrations -database "$(DATABASE_URL)" down 1
migrate-status: ## Show current migration version
	@$(MIGRATE) -path backend/migrations -database "$(DATABASE_URL)" version
migrate-create: ## Create a new migration pair: make migrate-create NAME=add_foo
	@$(MIGRATE) create -ext sql -dir backend/migrations -seq $(NAME)
