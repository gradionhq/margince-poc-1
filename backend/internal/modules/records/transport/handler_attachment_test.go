package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/records/domain"
	"github.com/gradionhq/margince/backend/internal/platform/blobstore"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/crmctx"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

const testWS = "00000000-0000-0000-0000-000000000att"

func withAttachWorkspace(r *http.Request) *http.Request {
	ctx := crmctx.With(r.Context(), crmctx.Principal{TenantID: testWS, UserID: "human:test"})
	return r.WithContext(ctx)
}

// fakeAttachmentStore satisfies attachmentStoreSeam with an in-memory map.
type fakeAttachmentStore struct {
	items   map[string]domain.Attachment
	nextErr error
}

func newFakeAttachmentStore() *fakeAttachmentStore {
	return &fakeAttachmentStore{items: make(map[string]domain.Attachment)}
}

func (f *fakeAttachmentStore) Create(_ context.Context, a domain.Attachment) (domain.Attachment, error) {
	if f.nextErr != nil {
		err := f.nextErr
		f.nextErr = nil
		return domain.Attachment{}, err
	}
	if a.ID == "" {
		a.ID = "att-1"
	}
	a.WorkspaceID = testWS
	a.CreatedAt = time.Now()
	f.items[a.ID] = a
	return a, nil
}

func (f *fakeAttachmentStore) Get(_ context.Context, id, _ string) (domain.Attachment, error) {
	if a, ok := f.items[id]; ok && a.ArchivedAt == nil {
		return a, nil
	}
	return domain.Attachment{}, errs.ErrNotFound
}

// GetAny returns the row regardless of archived_at status, mirroring
// production AttachmentStore.GetAny (no archived-at filter) — used by the
// transport get() handler so archived attachments stay retrievable.
func (f *fakeAttachmentStore) GetAny(_ context.Context, id, _ string) (domain.Attachment, error) {
	if a, ok := f.items[id]; ok {
		return a, nil
	}
	return domain.Attachment{}, errs.ErrNotFound
}

func (f *fakeAttachmentStore) List(_ context.Context, _, _, _, _ string, _ int, includeArchived bool) ([]domain.Attachment, string, error) {
	var out []domain.Attachment
	for _, a := range f.items {
		if !includeArchived && a.ArchivedAt != nil {
			continue
		}
		out = append(out, a)
	}
	return out, "", nil
}

func (f *fakeAttachmentStore) Archive(_ context.Context, id, _ string) (domain.Attachment, error) {
	if a, ok := f.items[id]; ok {
		now := time.Now()
		a.ArchivedAt = &now
		f.items[id] = a
		return a, nil
	}
	return domain.Attachment{}, errs.ErrNotFound
}

// fakeAudit records WriteAudit calls for assertion.
type fakeAudit struct{ called int }

func (f *fakeAudit) WriteAudit(_ context.Context, _, _, _, _ string) error {
	f.called++
	return nil
}

// newTestHandler builds an AttachmentHandler with nil db (visible=always-true
// default) and a MemoryStore blobstore. The isVisible field may be overridden
// after construction for visibility-gate tests.
func newTestHandler(store attachmentStoreSeam, audit *fakeAudit) *AttachmentHandler {
	blob := blobstore.NewMemoryStore()
	// db=nil → isVisible stays nil → withURLs treats every row as visible.
	return NewAttachmentHandler(store, blob, audit, nil)
}

// seed places a ready-made attachment in the fake store and returns its ID.
func seed(f *fakeAttachmentStore, scanStatus string) string {
	a := domain.Attachment{
		ID:          "att-fixed",
		WorkspaceID: testWS,
		EntityType:  "deal",
		EntityID:    "deal-1",
		Filename:    "report.pdf",
		ContentType: "application/pdf",
		ByteSize:    1024,
		StorageKey:  "attachments/" + testWS + "/att-fixed/report.pdf",
		ScanStatus:  scanStatus,
		Source:      "test",
		CapturedBy:  "human:test",
		CreatedAt:   time.Now(),
	}
	f.items[a.ID] = a
	return a.ID
}

// ---------------------------------------------------------------------------
// createAttachment tests
// ---------------------------------------------------------------------------

func TestAttachmentHandler_Create_Returns201WithUploadURL(t *testing.T) {
	store := newFakeAttachmentStore()
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)

	body := map[string]any{
		"entity_type":  "deal",
		"entity_id":    "deal-1",
		"filename":     "contract.pdf",
		"content_type": "application/pdf",
		"byte_size":    2048,
		"source":       "ui",
		"captured_by":  "human:test",
	}
	b, _ := json.Marshal(body)
	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments", bytes.NewReader(b)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Location") == "" {
		t.Fatal("want Location header on 201")
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	uploadURL, _ := resp["upload_url"].(string)
	if !strings.HasPrefix(uploadURL, "memory://") {
		t.Fatalf("want memory:// upload_url, got %q", uploadURL)
	}
	if scanStatus, _ := resp["scan_status"].(string); scanStatus != domain.ScanStatusScanning {
		t.Fatalf("want scan_status=scanning on create, got %q", scanStatus)
	}
	if resp["download_url"] != nil {
		t.Fatalf("download_url must be nil on create, got %v", resp["download_url"])
	}
}

func TestAttachmentHandler_Create_MissingProvenance_Returns422(t *testing.T) {
	store := newFakeAttachmentStore()
	h := newTestHandler(store, &fakeAudit{})

	body := map[string]any{
		"entity_type":  "deal",
		"entity_id":    "deal-1",
		"filename":     "x.pdf",
		"content_type": "application/pdf",
		"byte_size":    1,
		// source and captured_by missing
	}
	b, _ := json.Marshal(body)
	req := withAttachWorkspace(httptest.NewRequest(http.MethodPost, "/attachments", bytes.NewReader(b)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// getAttachment tests
// ---------------------------------------------------------------------------

func TestAttachmentHandler_Get_ScanningRow_DownloadURLNull(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusScanning)
	h := newTestHandler(store, &fakeAudit{})

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["download_url"] != nil {
		t.Fatalf("download_url must be nil for scanning row, got %v", resp["download_url"])
	}
}

func TestAttachmentHandler_Get_CleanVisible_ReturnsDownloadURLAndTriggersAudit(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)
	// isVisible already nil → defaults to visible=true

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	dlURL, _ := resp["download_url"].(string)
	if !strings.HasPrefix(dlURL, "memory://") {
		t.Fatalf("want memory:// download_url for clean+visible row, got %q", dlURL)
	}
	if audit.called != 1 {
		t.Fatalf("want exactly 1 audit write, got %d", audit.called)
	}
}

func TestAttachmentHandler_Get_BlockedRow_DownloadURLNull(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusBlocked)
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["download_url"] != nil {
		t.Fatalf("download_url must be nil for blocked row, got %v", resp["download_url"])
	}
	if audit.called != 0 {
		t.Fatalf("want 0 audit writes for blocked row, got %d", audit.called)
	}
}

func TestAttachmentHandler_Get_NotVisible_DisclosedLocked(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)
	// Override visibility gate to report "not visible".
	h.isVisible = func(_ context.Context, _, _, _ string, _ crmctx.Principal) (bool, error) {
		return false, nil
	}

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	// 200 not 404 — disclosed-locked row.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (disclosed-locked), body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["download_url"] != nil {
		t.Fatalf("download_url must be nil for not-visible row, got %v", resp["download_url"])
	}
	if resp["upload_url"] != nil {
		t.Fatalf("upload_url must be nil for not-visible row, got %v", resp["upload_url"])
	}
	// Full metadata still present.
	if resp["id"] == nil || resp["filename"] == nil {
		t.Fatalf("disclosed-locked row must carry full metadata, got %v", resp)
	}
	if audit.called != 0 {
		t.Fatalf("want 0 audit writes for not-visible row, got %d", audit.called)
	}
}

func TestAttachmentHandler_Get_MissingID_Returns404(t *testing.T) {
	store := newFakeAttachmentStore()
	h := newTestHandler(store, &fakeAudit{})

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/nonexistent", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", w.Code, w.Body.String())
	}
}

// Regression test for a live-stack UAT bug: GET /attachments/{id} on an
// archived attachment must return 200 (disclosed-locked, soft-deleted row —
// matching GET /organizations/{id}'s GetAny precedent), not 404. Both URLs
// must be nil and archived_at must be set.
func TestAttachmentHandler_Get_Archived_Returns200WithArchivedAtAndNullURLs(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	now := time.Now()
	a := store.items[id]
	a.ArchivedAt = &now
	store.items[id] = a
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (archived rows stay retrievable), body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["archived_at"] == nil {
		t.Fatal("want archived_at set for an archived row's GET response")
	}
	if resp["download_url"] != nil {
		t.Fatalf("download_url must be nil for an archived row, got %v", resp["download_url"])
	}
	if resp["upload_url"] != nil {
		t.Fatalf("upload_url must be nil for an archived row, got %v", resp["upload_url"])
	}
	if audit.called != 0 {
		t.Fatalf("want 0 audit writes for an archived row, got %d", audit.called)
	}
}

// Regression test for a live-stack UAT bug: attachmentResponse.ArchivedAt's
// `omitempty` tag dropped the required `archived_at` JSON key entirely
// instead of marshaling `null` for a live (non-archived) row. Assert the
// literal key is present in the marshaled bytes, not just that the Go struct
// field zero-values correctly (a struct-level check would not have caught
// this regression).
func TestAttachmentHandler_Get_ArchivedAtKeyAlwaysPresentInJSON(t *testing.T) {
	cases := []struct {
		name       string
		scanStatus string
		archive    bool
	}{
		{name: "active", scanStatus: domain.ScanStatusClean, archive: false},
		{name: "archived", scanStatus: domain.ScanStatusClean, archive: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeAttachmentStore()
			id := seed(store, tc.scanStatus)
			if tc.archive {
				now := time.Now()
				a := store.items[id]
				a.ArchivedAt = &now
				store.items[id] = a
			}
			h := newTestHandler(store, &fakeAudit{})

			req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments/"+id, nil))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte(`"archived_at"`)) {
				t.Fatalf("response JSON must always contain the archived_at key (schema requires it), got %s", w.Body.String())
			}

			var resp map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, present := resp["archived_at"]; !present {
				t.Fatalf("decoded JSON must contain the archived_at key, got %v", resp)
			}
			if tc.archive && resp["archived_at"] == nil {
				t.Fatal("want non-null archived_at for an archived row")
			}
			if !tc.archive && resp["archived_at"] != nil {
				t.Fatalf("want null archived_at for an active row, got %v", resp["archived_at"])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// listAttachments tests
// ---------------------------------------------------------------------------

func TestAttachmentHandler_List_CleanVisible_ReturnsDownloadURLNoSilentDrop(t *testing.T) {
	store := newFakeAttachmentStore()
	seed(store, domain.ScanStatusClean)
	// Also add a scanning one — both must appear in the list (never silently dropped).
	store.items["att-scan"] = domain.Attachment{
		ID: "att-scan", WorkspaceID: testWS, EntityType: "deal", EntityID: "deal-1",
		Filename: "draft.pdf", ContentType: "application/pdf", ByteSize: 512,
		StorageKey: "attachments/" + testWS + "/att-scan/draft.pdf",
		ScanStatus: domain.ScanStatusScanning, Source: "test", CapturedBy: "human:test",
		CreatedAt: time.Now(),
	}
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(page.Data) != 2 {
		t.Fatalf("want 2 items (no silent drop), got %d", len(page.Data))
	}
	// The clean one must have a download_url; the scanning one must not.
	var cleanURLs, scanURLs int
	for _, item := range page.Data {
		if item["scan_status"] == domain.ScanStatusClean && item["download_url"] != nil {
			cleanURLs++
		}
		if item["scan_status"] == domain.ScanStatusScanning && item["download_url"] == nil {
			scanURLs++
		}
	}
	if cleanURLs != 1 {
		t.Fatalf("want 1 clean row with download_url, got %d", cleanURLs)
	}
	if scanURLs != 1 {
		t.Fatalf("want 1 scanning row with nil download_url, got %d", scanURLs)
	}
	if audit.called != 1 {
		t.Fatalf("want exactly 1 audit write (only clean row), got %d", audit.called)
	}
}

func TestAttachmentHandler_List_NotVisible_DownloadURLNullNoAudit(t *testing.T) {
	store := newFakeAttachmentStore()
	seed(store, domain.ScanStatusClean)
	audit := &fakeAudit{}
	h := newTestHandler(store, audit)
	h.isVisible = func(_ context.Context, _, _, _ string, _ crmctx.Principal) (bool, error) {
		return false, nil
	}

	req := withAttachWorkspace(httptest.NewRequest(http.MethodGet, "/attachments", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &page)
	if len(page.Data) != 1 {
		t.Fatalf("want 1 item (never silently dropped), got %d", len(page.Data))
	}
	if page.Data[0]["download_url"] != nil {
		t.Fatalf("download_url must be nil for not-visible item, got %v", page.Data[0]["download_url"])
	}
	if audit.called != 0 {
		t.Fatalf("want 0 audit writes for not-visible, got %d", audit.called)
	}
}

// ---------------------------------------------------------------------------
// archiveAttachment tests
// ---------------------------------------------------------------------------

func TestAttachmentHandler_Archive_Returns200WithArchivedAt(t *testing.T) {
	store := newFakeAttachmentStore()
	id := seed(store, domain.ScanStatusClean)
	h := newTestHandler(store, &fakeAudit{})

	req := withAttachWorkspace(httptest.NewRequest(http.MethodDelete, "/attachments/"+id, nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["archived_at"] == nil {
		t.Fatal("want archived_at set in archive response")
	}
	if resp["download_url"] != nil || resp["upload_url"] != nil {
		t.Fatal("both URLs must be nil in archive response")
	}
}
