// Package domain holds the Attachment entity (RD-DDL-1) — a blob-store
// object reference bound to a record. Bytes never live here or in the DB;
// only metadata + the object-store key (ADR-0051).
package domain

import (
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// EntityType values an attachment may be bound to — the canonical
// polymorphic vocabulary (DM-CONV-17), matching attachment's own DB CHECK.
const (
	EntityTypePerson       = "person"
	EntityTypeOrganization = "organization"
	EntityTypeDeal         = "deal"
	EntityTypeLead         = "lead"
	EntityTypeActivity     = "activity"
)

// ScanStatus values (RD-PARAM-5). Scanning is the only state a fresh row
// starts in; only an explicit Scanner verdict moves it to Clean/Blocked —
// never silently.
const (
	ScanStatusScanning = "scanning"
	ScanStatusClean    = "clean"
	ScanStatusBlocked  = "blocked"
)

// Attachment is a blob-store object reference bound to a record (RD-DDL-1).
// No version/updated_at column exists — immutable except for archive.
type Attachment struct {
	ID          string
	WorkspaceID string
	EntityType  string
	EntityID    string
	Filename    string
	ContentType string
	ByteSize    int64
	StorageKey  string
	Checksum    *string
	ScanStatus  string
	Source      string
	CapturedBy  string
	CreatedAt   time.Time
	ArchivedAt  *time.Time
}

// NewAttachment returns an Attachment with a fresh ID, ScanStatusScanning,
// and copied provenance. storageKey must already be the caller-composed
// object key (Constraint 2: "attachments/<workspace_id>/<id>/<filename>").
func NewAttachment(entityType, entityID, filename, contentType string, byteSize int64, storageKey string, p prov.Provenance) Attachment {
	return Attachment{
		ID: ids.New(), EntityType: entityType, EntityID: entityID,
		Filename: filename, ContentType: contentType, ByteSize: byteSize,
		StorageKey: storageKey, ScanStatus: ScanStatusScanning,
		Source: p.Source, CapturedBy: p.CapturedBy,
	}
}
