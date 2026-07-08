package adapters

import (
	"context"
	"fmt"
	"time"

	actdomain "github.com/gradionhq/margince/backend/internal/modules/activities/domain"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
)

// activityCreator is the narrow interface WriteDownloadAudit requires from the
// activities module — matches *activitiesadapters.ActivityStore.Create exactly,
// avoiding a concrete-type import coupling beyond what's needed.
type activityCreator interface {
	Create(ctx context.Context, a actdomain.Activity) (actdomain.Activity, bool, error)
}

// WriteDownloadAudit records a getAttachment/listAttachments download as a
// timeline activity (RD-AC-2), reusing the activities module's own Create
// path rather than a bespoke audit mechanism. For person/organization/deal
// it links into activity_link so it appears on that record's 360 timeline
// (RD-AC-9); for lead/activity it writes an unlinked Activity row only —
// activity_link's own DB CHECK cannot bind those two types (accepted,
// documented gap, Constraint 5).
func WriteDownloadAudit(ctx context.Context, store activityCreator, workspaceID, entityType, entityID, filename string) error {
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
