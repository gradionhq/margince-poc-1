# ONA-T03 Live UAT Guide

Run this only after the stack is up with:

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

## Step 1: Verify the agents module stays green

```bash
go test ./backend/internal/modules/agents/...
```

Expected: every unit test in `backend/internal/modules/agents/...` passes, including the producer table tests from Tasks 1-4.

## Step 2: Exercise the reconciliation integration lane

```bash
make infra-up && make test-it DIR=backend/internal/modules/agents/app
```

Expected: the integration lane passes, including `TestRunPass_ReconciliationProduceEndToEnd` from Task 5.

## Step 3: Confirm migrations are unchanged

```bash
make migrate-status
```

Expected: the before/after migration status is unchanged for this branch, because ONA-T03 ships no migration.

## Step 4: Boot the real stack and confirm the module still compiles

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

Expected: the backend starts cleanly with the new producer files compiled in and no new startup error appears in the server logs.

```bash
go build ./backend/internal/modules/agents/...
```

Expected: the build succeeds.

## Step 5: Run the full gate

```bash
make check
```

Expected: the full gate exits 0.

