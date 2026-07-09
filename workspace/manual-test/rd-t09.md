# RD-T09 Live UAT Guide — Account Hierarchy Screen

Run only after the stack is up:

```bash
make infra-up && make migrate-up && make seed-reset && make run
```

---

## Step 1: Integration tests — org store Update with parent_org_id

```bash
make test-it DIR=backend/internal/modules/organizations
```

Expected: all three new tests in `store_org_update_test.go` pass:
- `TestOrgStore_Update_SetsParentOrgID` — sets parent_org_id and re-fetch confirms persistence
- `TestOrgStore_Update_RejectsCyclicParent` — returns `ErrOrganizationCycle` for a cycle
- `TestOrgStore_Update_ReparentsOrg` — re-parents correctly from one parent to another

---

## Step 2: Backend API — PATCH /organizations/:id rejects cycles

Seed two orgs `a` and `b` (use the CRM UI or the API directly). Set b's parent to a:

```bash
curl -s -X PATCH http://localhost:8080/api/v1/organizations/<b-id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"parent_org_id": "<a-id>", "version": 1}'
```

Expected: 200 OK — b is now a child of a.

Now attempt the cycle (a → b, which would create a → b → a):

```bash
curl -s -X PATCH http://localhost:8080/api/v1/organizations/<a-id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"parent_org_id": "<b-id>", "version": 1}'
```

Expected: 422 Unprocessable Entity with body:

```json
{
  "errors": [
    { "field": "parent_org_id", "code": "organization_cycle" }
  ]
}
```

---

## Step 3: Navigate to the Account Hierarchy screen

1. Open `http://localhost:5173` and log in.
2. Go to **Companies** and open any company that has child accounts (or create one with `parent_org_id` set via the PATCH API).
3. Append `/hierarchy` to the URL: `http://localhost:5173/companies/<id>/hierarchy`.

Expected:
- Page title shows "Account Hierarchy — <Company Name>"
- RollupTilesBand shows weighted pipeline, closed-won, 30d activity count, and account count for the tree.
- "Shows up to 200 accounts" caption appears below the tree table.
- AccountTree renders nested rows with expand/collapse twist buttons on parent nodes.

---

## Step 4: Scope toggle

On the Account Hierarchy page, click **"This account only (self)"**.

Expected:
- The tile figures change to the self-scope values (this account's contribution only).
- The "aggregated over N accounts" badge shows 1 (or the self-count if the org is also a parent).

Click back to **"Whole tree (roll-up)"**.

Expected: figures revert to tree-aggregated values.

---

## Step 5: Explain box

Click the **"Explain this roll-up"** toggle/link on the page.

Expected:
- Formula `roll-up(node) = self(node) + Σ roll-up(child)` is shown.
- Self figure and Children sum figure are displayed.
- If any restricted_excluded entries exist, they are listed here.

---

## Step 6: Restrict a child account's visibility

(Requires a second CRM user with limited permissions, or adjust workspace member roles.)

If a child org is restricted to the current user, open its hierarchy page.

Expected:
- A **"Restricted"** section appears below the main tree.
- Restricted rows show a lock icon, masked financial figures (`FieldGuard mode="masked"`), and the note "Excluded from roll-up (no access)".
- The restricted org does NOT appear in the main tree rows.

---

## Step 7: Suggested edge card

1. Create a new org with `parent_org_id: null` but sharing the same primary domain as the root org (e.g. `acme.com`).
2. Navigate back to the root org's hierarchy page.

Expected:
- A **"Suggested connections"** section appears with a card for the orphan org.
- The card shows the candidate's name and "Accept edge" + "Dismiss" buttons.

Click **Accept edge**:
- The card flips to "edge written · audited" text.
- The treeOrgs cache is invalidated; the newly linked org appears in the tree on next render.

Click **Dismiss** on a different candidate card:
- The card disappears immediately.
- No PATCH request is sent.

---

## Step 8: Empty state

Navigate to the hierarchy page of an org that has no children and no restricted nodes.

Expected:
- The message "No sub-accounts in this hierarchy yet." is displayed.
- The tree table and suggested-connections section do not render.

---

## Step 9: Full gate

```bash
pnpm --dir frontend biome check
pnpm --dir frontend test
make check-go
make test-it DIR=backend/internal/modules/organizations
make check
```

Expected: all gates exit 0.
