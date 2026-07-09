# ONA-T03 Live UAT Guide

Run this only after the stack is up with:

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

The commands below assume:

- the API is listening on `http://localhost:8080`
- the database is reachable through `DATABASE_URL`
- you can act as a tenant by sending `X-Workspace-Id` + `X-User-ID`

Use one shell session so the variables exported in the bootstrap block stay available for the later steps.

## Bootstrap

```bash
export API_BASE='http://localhost:8080'
export WS_ID='00000000-0000-0000-0000-0000000000a3'
export USER_ID='00000000-0000-0000-00a3-000000000001'
export ROLE_ID='00000000-0000-0000-00a3-000000000010'
export DEAL_ID='00000000-0000-0000-00a3-000000000020'
export ORG_ID='00000000-0000-0000-00a3-000000000030'
```

Expected: the variables are exported in the shell that will run the guide.

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

