package adapters

import (
	"context"
	"fmt"
	"time"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

const (
	activityKindNote     = "note"
	activitySourceSystem = "system"
)

// ActivityCreator is the narrow interface WriteDownloadAudit (and
// DownloadAuditWriter) require from the activities module — matches
// *activitiesadapters.ActivityStore.Create exactly, avoiding a concrete-type
// import coupling beyond what's needed.
type ActivityCreator interface {
	Create(ctx context.Context, a actdomain.Activity) (actdomain.Activity, bool, error)
}

// DownloadAuditWriter wraps WriteDownloadAudit as a concrete exported type so
// the transport layer can accept a clean interface without importing activities
// domain types directly (avoids adding activitiesdomain to recordstransport's
// arch-lint dep list).
type DownloadAuditWriter struct{ store ActivityCreator }

// NewDownloadAuditWriter constructs a DownloadAuditWriter from any ActivityCreator.
func NewDownloadAuditWriter(store ActivityCreator) *DownloadAuditWriter {
	return &DownloadAuditWriter{store: store}
}

// WriteAudit implements the transport layer's auditSeam interface.
func (w *DownloadAuditWriter) WriteAudit(ctx context.Context, workspaceID, entityType, entityID, filename string) error {
	return WriteDownloadAudit(ctx, w.store, workspaceID, entityType, entityID, filename)
}

// WriteRequestAccessAudit implements the transport layer's auditSeam interface.
func (w *DownloadAuditWriter) WriteRequestAccessAudit(ctx context.Context, workspaceID, entityType, entityID, filename string) error {
	return WriteRequestAccessAudit(ctx, w.store, workspaceID, entityType, entityID, filename)
}

// WriteExtractionAcceptAudit implements the transport layer's auditSeam interface.
func (w *DownloadAuditWriter) WriteExtractionAcceptAudit(ctx context.Context, workspaceID, entityType, entityID, field, sourceQuote, capturedBy string) error {
	return WriteExtractionAcceptAudit(ctx, w.store, workspaceID, entityType, entityID, field, sourceQuote, capturedBy)
}

// WriteDownloadAudit records a getAttachment/listAttachments download as a
// timeline activity (RD-AC-2), reusing the activities module's own Create
// path rather than a bespoke audit mechanism. For person/organization/deal
// it links into activity_link so it appears on that record's 360 timeline
// (RD-AC-9); for lead/activity it writes an unlinked Activity row only —
// activity_link's own DB CHECK cannot bind those two types (accepted,
// documented gap, Constraint 5).
func WriteDownloadAudit(ctx context.Context, store ActivityCreator, workspaceID, entityType, entityID, filename string) error {
	subject := fmt.Sprintf("Attachment downloaded: %s", filename)
	a := actdomain.Activity{
		WorkspaceID: workspaceID,
		Kind:        activityKindNote,
		Subject:     &subject,
		OccurredAt:  time.Now().UTC(),
		Source:      activitySourceSystem,
		CapturedBy:  "system:attachment-download-audit",
	}
	switch entityType {
	case domain.EntityTypePerson, domain.EntityTypeOrganization, domain.EntityTypeDeal:
		a.Links = []actdomain.ActivityLink{{EntityType: entityType, EntityID: entityID}}
	}
	_, _, err := store.Create(ctx, a)
	return err
}

// WriteRequestAccessAudit records a request-access click as an activity note.
func WriteRequestAccessAudit(ctx context.Context, store ActivityCreator, workspaceID, entityType, entityID, filename string) error {
	subject := fmt.Sprintf("Access requested: %s", filename)
	a := actdomain.Activity{
		WorkspaceID: workspaceID,
		Kind:        activityKindNote,
		Subject:     &subject,
		OccurredAt:  time.Now().UTC(),
		Source:      activitySourceSystem,
		CapturedBy:  requestCapturedBy(ctx),
	}
	switch entityType {
	case domain.EntityTypePerson, domain.EntityTypeOrganization, domain.EntityTypeDeal:
		a.Links = []actdomain.ActivityLink{{EntityType: entityType, EntityID: entityID}}
	}
	_, _, err := store.Create(ctx, a)
	return err
}

// WriteExtractionAcceptAudit records one accepted extraction field as a note.
func WriteExtractionAcceptAudit(ctx context.Context, store ActivityCreator, workspaceID, entityType, entityID, field, sourceQuote, capturedBy string) error {
	subject := fmt.Sprintf("Extraction accepted: %s", field)
	a := actdomain.Activity{
		WorkspaceID: workspaceID,
		Kind:        activityKindNote,
		Subject:     &subject,
		Body:        &sourceQuote,
		OccurredAt:  time.Now().UTC(),
		Source:      activitySourceSystem,
		CapturedBy:  capturedBy,
	}
	switch entityType {
	case domain.EntityTypePerson, domain.EntityTypeOrganization, domain.EntityTypeDeal:
		a.Links = []actdomain.ActivityLink{{EntityType: entityType, EntityID: entityID}}
	}
	_, _, err := store.Create(ctx, a)
	return err
}

func requestCapturedBy(ctx context.Context) string {
	if p, ok := crmctx.From(ctx); ok && p.UserID != "" {
		return "human:" + p.UserID
	}
	return "system:attachment-request-access"
}
