package transport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/records/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	platformauth "github.com/gradionhq/margince/backend/internal/platform/auth"
	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/httpkit"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

// attachmentStoreSeam is the subset of *adapters.AttachmentStore this handler needs.
type attachmentStoreSeam interface {
	Create(ctx context.Context, a domain.Attachment) (domain.Attachment, error)
	Get(ctx context.Context, id, workspaceID string) (domain.Attachment, error)
	// GetAny returns the row regardless of archived_at status — used by the
	// single-item GET handler so archived attachments stay retrievable.
	GetAny(ctx context.Context, id, workspaceID string) (domain.Attachment, error)
	List(ctx context.Context, workspaceID, entityType, entityID, cursor string, limit int, includeArchived bool) ([]domain.Attachment, string, error)
	Archive(ctx context.Context, id, workspaceID string) (domain.Attachment, error)
}

// auditSeam abstracts the download-audit write so this package does not import
// activities domain types directly (Constraint 5). The concrete implementation
// is *adapters.DownloadAuditWriter.
type auditSeam interface {
	WriteAudit(ctx context.Context, workspaceID, entityType, entityID, filename string) error
}

// AttachmentHandler routes /attachments and /attachments/{id} requests.
type AttachmentHandler struct {
	store attachmentStoreSeam
	blob  blobstore.Store
	audit auditSeam
	// isVisible combines LoadRolePermissions + RecordVisible (Constraint 6). When
	// nil (unit tests without a real DB) every attachment is treated as visible.
	isVisible func(ctx context.Context, wsID, entityType, entityID string, principal crmctx.Principal) (bool, error)
}

// NewAttachmentHandler returns an AttachmentHandler. db is used to load role
// permissions and check bound-record visibility (Constraint 6); pass nil only
// in unit tests — visibility defaults to true when db is nil.
func NewAttachmentHandler(store attachmentStoreSeam, blob blobstore.Store, audit auditSeam, db *sql.DB) *AttachmentHandler {
	h := &AttachmentHandler{store: store, blob: blob, audit: audit}
	if db != nil {
		h.isVisible = func(ctx context.Context, wsID, entityType, entityID string, principal crmctx.Principal) (bool, error) {
			perms, err := platformauth.LoadRolePermissions(ctx, db, wsID, principal.UserID)
			if err != nil {
				return false, err
			}
			return adapters.RecordVisible(ctx, db, wsID, entityType, entityID, principal, perms)
		}
	}
	return h
}

// ServeHTTP dispatches /attachments and /attachments/{id}.
func (h *AttachmentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := httpkit.PathID(r.URL.Path, "/attachments")
	switch {
	case r.Method == http.MethodGet && id == "":
		h.list(w, r)
	case r.Method == http.MethodPost && id == "":
		h.create(w, r)
	case r.Method == http.MethodGet && id != "":
		h.get(w, r, id)
	case r.Method == http.MethodDelete && id != "":
		h.archive(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

// attachmentResponse is the JSON wire shape — domain fields plus the two
// ephemeral presigned-URL fields the domain struct deliberately omits
// (ADR-0051: URLs are time-limited tokens, never stored in the DB).
type attachmentResponse struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	EntityType  string     `json:"entity_type"`
	EntityID    string     `json:"entity_id"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	ByteSize    int64      `json:"byte_size"`
	StorageKey  string     `json:"storage_key"`
	Checksum    *string    `json:"checksum,omitempty"`
	ScanStatus  string     `json:"scan_status"`
	Source      string     `json:"source"`
	CapturedBy  string     `json:"captured_by"`
	CreatedAt   time.Time  `json:"created_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
	UploadURL   *string    `json:"upload_url,omitempty"`
	DownloadURL *string    `json:"download_url,omitempty"`
}

func toResponse(a domain.Attachment, uploadURL, downloadURL *string) attachmentResponse {
	return attachmentResponse{
		ID: a.ID, WorkspaceID: a.WorkspaceID, EntityType: a.EntityType, EntityID: a.EntityID,
		Filename: a.Filename, ContentType: a.ContentType, ByteSize: a.ByteSize,
		StorageKey: a.StorageKey, Checksum: a.Checksum, ScanStatus: a.ScanStatus,
		Source: a.Source, CapturedBy: a.CapturedBy, CreatedAt: a.CreatedAt, ArchivedAt: a.ArchivedAt,
		UploadURL: uploadURL, DownloadURL: downloadURL,
	}
}

// createAttachmentRequest is the decoded POST /attachments body.
type createAttachmentRequest struct {
	EntityType  string  `json:"entity_type"`
	EntityID    string  `json:"entity_id"`
	Filename    string  `json:"filename"`
	ContentType string  `json:"content_type"`
	ByteSize    int64   `json:"byte_size"`
	Checksum    *string `json:"checksum,omitempty"`
	Source      string  `json:"source"`
	CapturedBy  string  `json:"captured_by"`
}

const presignExpiry = 15 * time.Minute

// fieldRequired is the FieldError code used for every "missing required
// field" validation failure in this handler.
const fieldRequired = "required"

func (h *AttachmentHandler) create(w http.ResponseWriter, r *http.Request) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	var body createAttachmentRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpkit.JSONProblem(w, http.StatusBadRequest, "bad_request")
		return
	}

	var ferrs []httpkit.FieldError
	if body.EntityType == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "entity_type", Code: fieldRequired})
	}
	if body.EntityID == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "entity_id", Code: fieldRequired})
	}
	if body.Filename == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "filename", Code: fieldRequired})
	}
	if body.ContentType == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "content_type", Code: fieldRequired})
	}
	if body.ByteSize <= 0 {
		ferrs = append(ferrs, httpkit.FieldError{Field: "byte_size", Code: fieldRequired})
	}
	if body.Source == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "source", Code: fieldRequired})
	}
	if body.CapturedBy == "" {
		ferrs = append(ferrs, httpkit.FieldError{Field: "captured_by", Code: fieldRequired})
	}
	if len(ferrs) > 0 {
		httpkit.JSONValidationError(w, "entity_type, entity_id, filename, content_type, byte_size, source, and captured_by are required.", ferrs)
		return
	}

	// Mint the Attachment (fresh ID generated inside domain.NewAttachment), then
	// compose the storage key from it (Constraint 2).
	a := domain.NewAttachment(
		body.EntityType, body.EntityID, body.Filename, body.ContentType,
		body.ByteSize, "", prov.Provenance{Source: body.Source, CapturedBy: body.CapturedBy},
	)
	a.WorkspaceID = wsID
	a.Checksum = body.Checksum
	a.StorageKey = fmt.Sprintf("attachments/%s/%s/%s", wsID, a.ID, body.Filename)

	created, err := h.store.Create(r.Context(), a)
	if errors.Is(err, errs.ErrNullProvenance) {
		httpkit.JSONValidationError(w, "source and captured_by are required.",
			[]httpkit.FieldError{{Field: "source", Code: fieldRequired}, {Field: "captured_by", Code: fieldRequired}})
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}

	uploadURL, err := h.blob.PresignedPutURL(r.Context(), created.StorageKey, presignExpiry)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	// download_url is always nil on create: a fresh row is scan_status='scanning'.
	httpkit.JSONCreatedAt(w, toResponse(created, &uploadURL, nil), "/attachments/"+created.ID)
}

func (h *AttachmentHandler) get(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	// GetAny (not Get): an archived attachment must stay retrievable via a
	// plain GET — soft-delete convention, matching GET /organizations/{id}.
	a, err := h.store.GetAny(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	resp, err := h.withURLs(r, wsID, a)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	httpkit.JSONOK(w, resp)
}

func (h *AttachmentHandler) list(w http.ResponseWriter, r *http.Request) {
	wsID, ok := httpkit.RequireWorkspace(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	includeArchived := q.Get("include_archived") == "true"
	items, next, err := h.store.List(r.Context(), wsID, q.Get("entity_type"), q.Get("entity_id"), q.Get("cursor"), httpkit.QueryLimit(r, 20), includeArchived)
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	resps := make([]attachmentResponse, 0, len(items))
	for _, a := range items {
		resp, err := h.withURLs(r, wsID, a)
		if err != nil {
			httpkit.JSONError(w, err)
			return
		}
		resps = append(resps, resp)
	}
	httpkit.JSONOK(w, httpkit.PageResponse(resps, next))
}

// withURLs applies the visibility gate (Constraint 6), the archived gate, and
// the scan-status gate (Constraint 5) to a single attachment row. Returns a
// "disclosed-locked" row (both URLs nil, no audit write) when the bound
// record is not visible, the attachment is archived, or it is not yet
// scan_status='clean'. Never drops the row or returns 404.
func (h *AttachmentHandler) withURLs(r *http.Request, wsID string, a domain.Attachment) (attachmentResponse, error) {
	if h.isVisible != nil {
		principal, _ := crmctx.From(r.Context())
		visible, err := h.isVisible(r.Context(), wsID, a.EntityType, a.EntityID, principal)
		if err != nil {
			return attachmentResponse{}, err
		}
		if !visible {
			return toResponse(a, nil, nil), nil
		}
	}

	// Archived = soft-deleted: never downloadable, regardless of scan_status
	// (matches archiveAttachment's own response and the "no bytes access
	// implied" convention).
	if a.ArchivedAt != nil {
		return toResponse(a, nil, nil), nil
	}

	if a.ScanStatus != domain.ScanStatusClean {
		return toResponse(a, nil, nil), nil
	}

	downloadURL, err := h.blob.PresignedGetURL(r.Context(), a.StorageKey, presignExpiry)
	if err != nil {
		return attachmentResponse{}, err
	}
	if err := h.audit.WriteAudit(r.Context(), wsID, a.EntityType, a.EntityID, a.Filename); err != nil {
		return attachmentResponse{}, err
	}
	return toResponse(a, nil, &downloadURL), nil
}

func (h *AttachmentHandler) archive(w http.ResponseWriter, r *http.Request, id string) {
	wsID := httpkit.WorkspaceID(r)
	a, err := h.store.Archive(r.Context(), id, wsID)
	if errors.Is(err, errs.ErrNotFound) {
		httpkit.JSONProblem(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		httpkit.JSONError(w, err)
		return
	}
	// Archived attachments: both URLs nil — soft-deleted, no bytes access implied.
	httpkit.JSONOK(w, toResponse(a, nil, nil))
}
