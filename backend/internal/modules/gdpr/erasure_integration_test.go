//go:build integration

package crmgdpr_test

import (
	"context"
	"database/sql"
	"testing"

	crmgdpr "github.com/gradionhq/margince/backend/internal/modules/gdpr"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
)

// zeroVec returns a SQL literal string of n zeros, e.g. '[0,0,...,0]'.
// Safe to embed directly in query strings (only digits and [,]).
func zeroVec(n int) string {
	b := make([]byte, 0, 2+n*2)
	b = append(b, '[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '0')
	}
	b = append(b, ']')
	return "'" + string(b) + "'"
}

// TestErase_NormalizedPIIRemoved verifies that Erase:
//   - nulls PII on person (first_name, last_name, raw) and sets full_name='[erased]'
//   - deletes all person_email rows
//   - nulls subject/body/raw on linked activities
//   - writes exactly one PII-free audit_log row (action=erase, before IS NULL, after IS NULL)
func TestErase_NormalizedPIIRemoved(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Seed PII on the person row.
	mustExec(t, db, `UPDATE person SET first_name='Alice', last_name='Smith',
		raw='{"email":"alice@example.com"}'::jsonb WHERE id=$1::uuid`, personID)

	// Seed a person_email row.
	mustExec(t, db, `INSERT INTO person_email (id,workspace_id,person_id,email,source,captured_by)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'alice@example.com','test','test')`,
		ids.New(), wsID, personID)

	// Seed an activity with PII, linked to the person.
	activityID := ids.New()
	mustExec(t, db, `INSERT INTO activity (id,workspace_id,kind,subject,body,raw,source,captured_by)
		VALUES ($1::uuid,$2::uuid,'note','Meeting with Alice','Confidential body','{"pii":"data"}'::jsonb,'test','test')`,
		activityID, wsID)
	mustExec(t, db, `INSERT INTO activity_link (id,workspace_id,activity_id,entity_type,person_id)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'person',$4::uuid)`,
		ids.New(), wsID, activityID, personID)

	// Call the actual exported symbol under test.
	if err := crmgdpr.Erase(ctx, db, personID); err != nil {
		t.Fatalf("Erase: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Assert person PII is nulled and full_name set to tombstone value.
	var firstName, lastName sql.NullString
	var rawCol sql.NullString
	var fullName string
	if err := db.QueryRow(
		`SELECT first_name, last_name, full_name, raw::text FROM person WHERE id=$1::uuid`, personID,
	).Scan(&firstName, &lastName, &fullName, &rawCol); err != nil {
		t.Fatalf("query person: %v", err)
	}
	if firstName.Valid {
		t.Errorf("first_name should be NULL after erase, got %q", firstName.String)
	}
	if lastName.Valid {
		t.Errorf("last_name should be NULL after erase, got %q", lastName.String)
	}
	if fullName != "[erased]" {
		t.Errorf("full_name: want [erased], got %q", fullName)
	}
	if rawCol.Valid {
		t.Errorf("person.raw should be NULL after erase, got %q", rawCol.String)
	}

	// Assert person_email rows are deleted.
	var emailCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM person_email WHERE person_id=$1::uuid`, personID,
	).Scan(&emailCount); err != nil {
		t.Fatalf("query person_email: %v", err)
	}
	if emailCount != 0 {
		t.Errorf("person_email count: want 0, got %d", emailCount)
	}

	// Assert linked activity PII is nulled.
	var actSubject, actBody, actRaw sql.NullString
	if err := db.QueryRow(
		`SELECT subject, body, raw::text FROM activity WHERE id=$1::uuid`, activityID,
	).Scan(&actSubject, &actBody, &actRaw); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actSubject.Valid {
		t.Errorf("activity.subject should be NULL, got %q", actSubject.String)
	}
	if actBody.Valid {
		t.Errorf("activity.body should be NULL, got %q", actBody.String)
	}
	if actRaw.Valid {
		t.Errorf("activity.raw should be NULL, got %q", actRaw.String)
	}

	// Assert exactly one PII-free erase tombstone in audit_log.
	var auditCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM audit_log
		 WHERE action='erase' AND entity_type='person'
		   AND entity_id=$1::uuid AND before IS NULL AND after IS NULL
		   AND workspace_id=$2::uuid`,
		personID, wsID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query audit_log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit_log erase rows: want 1, got %d", auditCount)
	}
}

// TestErase_EmbeddingDeleted verifies that Erase deletes embedding rows for the person.
func TestErase_EmbeddingDeleted(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Seed an embedding row for the person (1024-dim zero vector).
	mustExec(t, db, `INSERT INTO embedding (workspace_id, source_type, source_id, content_hash, embedding, dims, source, captured_by)
        VALUES ($1::uuid, 'person', $2::uuid, 'testhash', `+zeroVec(1024)+`::vector, 1024, 'test', 'test')`,
		wsID, personID)

	if err := crmgdpr.Erase(ctx, db, personID); err != nil {
		t.Fatalf("Erase: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	var n int
	if err := db.QueryRow(
		`SELECT count(*) FROM embedding WHERE source_type='person' AND source_id=$1::uuid`, personID,
	).Scan(&n); err != nil {
		t.Fatalf("query embedding: %v", err)
	}
	if n != 0 {
		t.Errorf("embedding rows: want 0 after erase, got %d", n)
	}
}

// TestErase_AllVectorTypesDeleted verifies erasure is TOTAL across pgvector:
// not just the person vector, but the subject's activity/lead/deal vectors too.
// gdpr.md promises erasure removes all of the subject's embeddings — a person-only
// delete (the pre-fix behavior) left activity/lead/deal text recoverable.
func TestErase_AllVectorTypesDeleted(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Lead the person was converted from.
	leadID := ids.New()
	mustExec(t, db, `INSERT INTO lead (id,workspace_id,full_name,status,score,source,captured_by,version)
		VALUES ($1::uuid,$2::uuid,'Alice Lead','new',0,'test','test',1)`, leadID, wsID)
	mustExec(t, db, `UPDATE person SET converted_from_lead_id=$1::uuid WHERE id=$2::uuid`, leadID, personID)

	// Activity linked to the person, and a deal (with its pipeline/stage) linked
	// via the same activity — the person → activity → deal chain Erase walks.
	activityID := ids.New()
	dealID := ids.New()
	pipelineID := ids.New()
	stageID := ids.New()
	mustExec(t, db, `INSERT INTO activity (id,workspace_id,kind,subject,source,captured_by)
		VALUES ($1::uuid,$2::uuid,'note','Meeting','test','test')`, activityID, wsID)
	mustExec(t, db, `INSERT INTO pipeline (id,workspace_id,name,is_default)
		VALUES ($1::uuid,$2::uuid,'P',true)`, pipelineID, wsID)
	mustExec(t, db, `INSERT INTO stage (id,workspace_id,pipeline_id,name,position)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'S',1)`, stageID, wsID, pipelineID)
	mustExec(t, db, `INSERT INTO deal (id,workspace_id,name,pipeline_id,stage_id,source,captured_by)
		VALUES ($1::uuid,$2::uuid,'Deal A',$3::uuid,$4::uuid,'test','test')`, dealID, wsID, pipelineID, stageID)
	mustExec(t, db, `INSERT INTO activity_link (id,workspace_id,activity_id,entity_type,person_id)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'person',$4::uuid)`, ids.New(), wsID, activityID, personID)
	mustExec(t, db, `INSERT INTO activity_link (id,workspace_id,activity_id,entity_type,deal_id)
		VALUES ($1::uuid,$2::uuid,$3::uuid,'deal',$4::uuid)`, ids.New(), wsID, activityID, dealID)

	// One embedding per source_type, all belonging to the subject.
	seedEmb := func(sourceType, srcID string) {
		mustExec(t, db, `INSERT INTO embedding (workspace_id, source_type, source_id, content_hash, embedding, dims, source, captured_by)
			VALUES ($1::uuid, $2, $3::uuid, $4, `+zeroVec(1024)+`::vector, 1024, 'test', 'test')`,
			wsID, sourceType, srcID, sourceType+"-hash")
	}
	seedEmb("person", personID)
	seedEmb("lead", leadID)
	seedEmb("activity", activityID)
	seedEmb("deal", dealID)

	if err := crmgdpr.Erase(ctx, db, personID); err != nil {
		t.Fatalf("Erase: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// Every one of the subject's vectors must be gone — none may survive.
	for _, st := range []string{"person", "lead", "activity", "deal"} {
		var n int
		if err := db.QueryRow(
			`SELECT count(*) FROM embedding WHERE workspace_id=$1::uuid AND source_type=$2`,
			wsID, st,
		).Scan(&n); err != nil {
			t.Fatalf("query embedding %s: %v", st, err)
		}
		if n != 0 {
			t.Errorf("embedding source_type=%q: want 0 after erase (erasure must be total), got %d", st, n)
		}
	}
}

// TestErase_SuppressionList verifies that Erase:
//   - adds email hashes to erasure_suppression (no plaintext stored)
//   - IsSuppressed returns true for the erased email
func TestErase_SuppressionList(t *testing.T) {
	db := testDB(t)
	wsID, personID := seedWorkspaceAndPerson(t, db)
	ctx := crmctx.With(context.Background(), crmctx.Principal{UserID: "system", TenantID: wsID})

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	const testEmail = "suppress-me@example.com"
	mustExec(t, db, `INSERT INTO person_email (id,workspace_id,person_id,email,source,captured_by)
        VALUES ($1::uuid,$2::uuid,$3::uuid,$4,'test','test')`,
		ids.New(), wsID, personID, testEmail)

	if err := crmgdpr.Erase(ctx, db, personID); err != nil {
		t.Fatalf("Erase: %v", err)
	}

	mustExec(t, db, `SELECT set_config('app.workspace_id',$1,false)`, wsID)

	// IsSuppressed must return true.
	suppressed, err := crmgdpr.IsSuppressed(ctx, db, wsID, testEmail)
	if err != nil {
		t.Fatalf("IsSuppressed: %v", err)
	}
	if !suppressed {
		t.Fatal("IsSuppressed: want true after Erase, got false")
	}

	// No plaintext email stored in erasure_suppression.
	var emailCount int
	if err := db.QueryRow(
		`SELECT count(*) FROM erasure_suppression WHERE email_hash=$1`,
		testEmail, // literal email — must NOT match hash
	).Scan(&emailCount); err != nil {
		t.Fatalf("query erasure_suppression plaintext: %v", err)
	}
	if emailCount != 0 {
		t.Error("erasure_suppression must not store plaintext email")
	}
}
