# Activities & Timeline chapter — manual guide (AT-T01..T05)

Checkpoint for GitHub epic **#83 — AT (Activities & Timeline)**. One ordered walkthrough for a
human tester verifying everything the Activities & Timeline chapter shipped:

- **AT-T02** — the activity store + `logActivity`/`getActivity`: idempotent capture-keyed create,
  typed multi-entity links, non-null provenance, raw JSONB off the hot path, task-field integrity.
- **AT-T03** — the `listActivities` timeline read: the closed DM-VOCAB-4 filter/sort vocabulary,
  full-text, cursor pagination.
- **AT-T04** — `updateActivity` (merge-patch, task→done) + `archiveActivity` (soft delete), each
  with exactly one audit row + the right event.
- **AT-T05** — `relinkActivity`: idempotent typed-link add/move, provenance byte-preserved, exactly
  one audit row.
- **AT-T01** — the `relinkActivity` contract + `activity_relink` audit action token in `crm.yaml`.

> **This chapter is API-first.** There is **no dedicated Activities screen** — activities *appear*
> read-only as timelines on the 360 views (Person → Activity tab, Company → activity card, Deal →
> Timeline card), and every write (`log`/`update`/`archive`/`relink`) is an API operation. So this
> guide is mostly `curl`, with the UI used to **confirm reads render honestly**. Read it top to
> bottom.

## Setup (do this once)

1. Boot the stack:
   ```bash
   make infra-up && make migrate-up && make seed-reset && make run
   ```
   **Expected:** API on `:8080`, dev seed loaded.
2. Start the frontend (for the read-side confirmations in Parts 1 & 6):
   ```bash
   make fe-dev
   ```
   **Expected:** Vite on `http://localhost:5173`; `/api` proxies to `:8080`.
3. Constants (from `backend/seed/dev.sql`) used throughout:

   | Name | Value |
   |---|---|
   | Workspace ID | `00000000-0000-0000-0000-000000000001` |
   | Admin user ID | `00000000-0000-0000-0010-000000000001` |
   | Alice Müller (person) | `00000000-0000-0000-0001-000000000001` |
   | API base | `http://localhost:8080` |

   Every write below sends the dev headers (exactly what the Vite proxy injects locally):
   ```bash
   -H 'Content-Type: application/json' \
   -H 'X-Workspace-ID: 00000000-0000-0000-0000-000000000001' \
   -H 'X-User-ID: 00000000-0000-0000-0010-000000000001'
   ```
   For brevity the steps below abbreviate these three as **`$HDRS`**. Export them once:
   ```bash
   HDRS=(-H 'Content-Type: application/json' \
     -H 'X-Workspace-ID: 00000000-0000-0000-0000-000000000001' \
     -H 'X-User-ID: 00000000-0000-0000-0010-000000000001')
   ```
   (then use `curl "${HDRS[@]}" ...` in zsh/bash).

4. Grab a seeded **deal id** for the multi-entity link test — open the Deals board and copy any
   deal's id from its URL, or `curl "${HDRS[@]}" http://localhost:8080/deals | jq '.data[0].id'`.
   Call it `<DEAL_ID>` below.

## Part 1 — Log an activity + the read timeline renders (AT-T02, AT-T03)

1. **Log one activity linked to two entities** — an email tied to both Alice and a deal (AT-T02
   `AC-1`). Note the `source_system`/`source_id` — that pair is the idempotency key we replay in
   Part 2:
   ```bash
   curl -s -X POST http://localhost:8080/activities "${HDRS[@]}" -d '{
     "kind":"email",
     "subject":"Renewal pricing follow-up",
     "occurred_at":"2026-07-01T09:00:00Z",
     "source":"connector",
     "captured_by":"agent:gmail",
     "source_system":"gmail",
     "source_id":"thread-AT-manual-001",
     "raw":{"snippet":"Sending the updated quote over."},
     "links":[
       {"entity_type":"person","entity_id":"00000000-0000-0000-0001-000000000001"},
       {"entity_type":"deal","entity_id":"<DEAL_ID>"}
     ]
   }'
   ```
   **Expected:** `201` with one activity carrying **two typed links** (person + deal), non-null
   `source`/`captured_by`, and the `raw` payload stored. Copy the returned id as `<ACT_ID>`.
2. **Read it back** (AT-T02 `WIRE-3`):
   ```bash
   curl -s http://localhost:8080/activities/<ACT_ID> "${HDRS[@]}" | jq
   ```
   **Expected:** one activity with its typed links **and** its raw capture payload. A read for a
   random/out-of-scope id answers **`404`**.
3. **Confirm it renders on the timeline (UI read side).** Open Alice Müller's Person 360
   (`http://localhost:5173/people/00000000-0000-0000-0001-000000000001`) → **Activity** tab.
   **Expected:** the email row shows **kind / subject / date** honestly, with the fixed provenance
   caption *"You logged none of this — every row carries its source."* — it does **not** fabricate a
   per-row source chip it doesn't have on the wire. Open the linked deal's 360 → **Timeline** card:
   **Expected:** the same activity appears there too (one activity, two entities).

## Part 2 — Idempotent capture (AT-T02 `AC-3`)

4. **Replay the exact same `(source_system, source_id)`** — re-run the step 1 `curl` verbatim.
   **Expected:** it resolves to the **existing** row and answers **`200`** (not `201`), returning
   the same activity id — **never a second activity**. This is the DB half of the bus dedupe rule,
   held by the `uq_activity_source` unique index.
5. Confirm no duplicate landed:
   ```bash
   curl -s 'http://localhost:8080/activities?entity_type=person&entity_id=00000000-0000-0000-0001-000000000001' "${HDRS[@]}" | jq '.data | length'
   ```
   **Expected:** the count did not double for that subject.

## Part 3 — The timeline read: vocabulary, full-text, pagination (AT-T03)

6. **Newest-first, cursor-paginated:**
   ```bash
   curl -s 'http://localhost:8080/activities?limit=5' "${HDRS[@]}" | jq '{count:(.data|length), next:.next_cursor}'
   ```
   **Expected:** up to 5 items newest-first plus a cursor; passing `?cursor=<next_cursor>` returns
   the next page.
7. **Filter by the closed DM-VOCAB-4 allow-list** — kind, linked entity, assignee, full-text:
   ```bash
   curl -s 'http://localhost:8080/activities?kind=email' "${HDRS[@]}" | jq '.data | length'
   curl -s 'http://localhost:8080/activities?q=renewal' "${HDRS[@]}" | jq '.data[].subject'
   ```
   **Expected:** filtered results; the full-text `q=renewal` matches step 1's subject.
8. **A key outside the vocabulary is refused, never silently ignored:**
   ```bash
   curl -s -o /dev/null -w '%{http_code}\n' 'http://localhost:8080/activities?sort=made_up_column' "${HDRS[@]}"
   ```
   **Expected:** **`422`** (`sort_field_not_allowed`) — the closed allow-list rejects an unknown
   sort/filter key rather than ignoring it (AT-T03 `WIRE-1`).

## Part 4 — Update + task→done + archive (AT-T04)

9. **Merge-patch an update** — correct the subject (a human correcting a captured field):
   ```bash
   curl -s -X PATCH http://localhost:8080/activities/<ACT_ID> "${HDRS[@]}" \
     -d '{"subject":"Renewal pricing follow-up (corrected)"}'
   ```
   **Expected:** `200`; only `subject` changes (merge-patch, not full replace). This emits exactly
   one **`activity.updated`** with typed-by attribution — **not** a generic created/updated
   double-fire (AT-T04 `WIRE-4`).
10. **Task→done transition.** Log a **task** kind, then transition it to done:
    ```bash
    ACT_TASK=$(curl -s -X POST http://localhost:8080/activities "${HDRS[@]}" \
      -d '{"kind":"task","subject":"Send updated quote","assignee_id":"00000000-0000-0000-0010-000000000001","source":"human","captured_by":"human:admin"}' | jq -r .id)
    curl -s -X PATCH http://localhost:8080/activities/$ACT_TASK "${HDRS[@]}" -d '{"is_done":true}'
    ```
    **Expected:** the done transition sets **`done_at`**, writes **exactly one audit row**, and
    emits **one `activity.updated`** carrying the done-state delta — the event is `activity.updated`,
    **not** `task.completed` (AT-T04 `AC-5`). (Task fields cannot persist on a non-task kind, and a
    done task always carries `done_at` — both DB-checked, per AT-T02 `AC-11`.)
11. **Archive (soft delete):**
    ```bash
    curl -s -X DELETE http://localhost:8080/activities/$ACT_TASK "${HDRS[@]}" | jq '{archived_at}'
    ```
    **Expected:** the entity is returned with its **`archived_at` set** (soft delete), and
    **`activity.archived`** is emitted — the row is not physically removed. It drops out of the
    default timeline; `?include_archived=true` brings it back.

## Part 5 — Relink (AT-T05, AT-T01)

12. **Add a typed link idempotently** — attach the step-1 email to a second person (or the same
    deal again to prove the no-op). Move/add via:
    ```bash
    curl -s -X POST http://localhost:8080/activities/<ACT_ID>/relink "${HDRS[@]}" \
      -d '{"entity_type":"deal","entity_id":"<DEAL_ID>"}'
    ```
    **Expected:** `200`; the response carries the link set. **Replay the same call** —
    **Expected:** it is a **no-op** against `uq_activity_link` and the link set is **unchanged**
    (AT-T05 `WIRE-6`). This is 🟢 (internal association, not an outbound action).
13. **Provenance is byte-preserved.** `GET /activities/<ACT_ID>` again. **Expected:** `source` and
    `captured_by` are **byte-identical** to Part 1 — relink records *who associated* the activity,
    never *who captured* it — and **exactly one audit row** records the relink (AT-T05 `AC-10`).
14. **The `activity_relink` audit action is admitted (AT-T01 / WIRE-N-2).** This is the check-sync
    migration proof — most cleanly verified by the coherence gate:
    ```bash
    bash scripts/check-audit-action-coherence.sh
    ```
    **Expected:** green — `audit_log_action_check` admits `activity_relink`, matching the contract
    action enum.

## Part 6 — Read-side honesty & STATE floor (empty timelines)

15. Open a Person 360 for someone with **no activities** (e.g. Carol Wagner,
    `00000000-0000-0000-0001-000000000003`) → Activity tab. **Expected:** an honest
    *"No activity captured yet."* empty state (STATE-1) — never a blank or a fabricated row.
16. Open a Deal 360 with no timeline entries → Timeline card. **Expected:** the honest
    *"You logged none of this"* empty message; a load error shows *"Failed to load activity."*, not
    a blanked screen (STATE-1 / STATE-3).

## Automated counterpart

Run these alongside (or instead of) the manual pass — they are the merge gate for every AT ticket
and cover the same ground at the API/DB level:

| Command | What it proves |
|---|---|
| `make check` | Format, lint, **codegen-drift** (AT-T01 `relinkActivity` contract + audit-action token in sync), DAG/invariants, plus `scripts/check-audit-action-coherence.sh` |
| `make test-contracts` | TypeScript contract-compliance against `crm.yaml` — AT-T01 wire shapes, API envelope/cursor/error conventions (AT-T03 `API-CONV-1..11`) |
| `make test-integration` | Go integration lane against seeded Postgres (`make infra-up` first) — AT-T02 idempotent capture + multi-entity links + raw-off-hot-path + provenance/task DB checks, AT-T03 vocabulary/full-text/pagination + **PERF-2/PERF-3 benchmarks** (p95 < 150 ms timeline / < 200 ms full-text), AT-T04 merge-patch/task-done/archive events, AT-T05 idempotent relink + byte-preserved provenance |

If a manual step doesn't match what you see:

1. Open the owning subsystem chapter `docs/subsystems/activities-and-timeline.md` for the full
   acceptance criteria and gate IDs (ACT-WIRE-*, ACT-AC-*, ACT-DDL-*, ACT-EVT-*).
2. `docs/quality/acceptance-standards.md` is the STATE-1..5 / performance floor the read surfaces
   inherit.
