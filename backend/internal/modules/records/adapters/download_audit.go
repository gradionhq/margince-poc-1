package adapters

import (
	"context"
	"fmt"
	"time"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
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
		Kind:        "note",
		Subject:     &subject,
		OccurredAt:  time.Now().UTC(),
		Source:      "system",
		CapturedBy:  "system:attachment-download-audit",
	}
	switch entityType {
	case domain.EntityTypePerson, domain.EntityTypeOrganization, domain.EntityTypeDeal:
		a.Links = []actdomain.ActivityLink{{EntityType: entityType, EntityID: entityID}}
	}
	_, _, err := store.Create(ctx, a)
	return err
}
