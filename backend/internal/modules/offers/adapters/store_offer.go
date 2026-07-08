package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/deals"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	"github.com/gradionhq/margince/backend/internal/modules/organizations"
	crmaudit "github.com/gradionhq/margince/backend/internal/platform/audit"
	database "github.com/gradionhq/margince/backend/internal/platform/database"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/ids"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/sqlutil"
)

const (
	entityTypeOffer = "offer"
	nilUUID         = "00000000-0000-0000-0000-000000000000"
)

// ErrDuplicateOfferNumber reports a live offer_number+revision collision
// within a workspace (offer_number_rev_unique — NOT a partial constraint, so
// this pre-check does not filter archived_at either), pre-checked ahead of
// the INSERT so the client gets 409 offer_number_duplicate, never a raw
// constraint error.
var ErrDuplicateOfferNumber = errors.New("duplicate offer_number+revision")

// ErrOfferNotDraft reports a mutation attempted against a non-draft offer
// (OFFER-WIRE-4) — returned by every offer/line-item write path.
var ErrOfferNotDraft = errors.New("offer is not draft")

// ErrOfferNotSent reports a mutation attempted against an offer that is not sent.
var ErrOfferNotSent = errors.New("offer is not sent")

// computeLineTotals derives one line's net/tax minor units from its
// quantity/unit_price_minor/discount_pct/tax_rate (OFFER-PARAM-4):
// line_net = round(quantity * unit_price_minor * (1 - discount_pct/100)),
// line_tax = round(line_net * tax_rate/100) — each rounded independently to
// the nearest minor unit BEFORE summing across lines (round-then-sum; never
// sum floats then round once, see OFFER-AC-3).
func computeLineTotals(quantity float64, unitPriceMinor int64, discountPct, taxRate float64) (lineNetMinor, lineTaxMinor int64) {
	net := quantity * float64(unitPriceMinor) * (1 - discountPct/100)
	lineNetMinor = int64(math.Round(net))
	lineTaxMinor = int64(math.Round(float64(lineNetMinor) * taxRate / 100))
	return lineNetMinor, lineTaxMinor
}

// recomputeOfferTotals re-derives offer.net_minor/tax_minor/gross_minor by
// summing every offer_line_item row's round-then-sum contribution
// (OFFER-AC-3) and persists the three columns. Must run inside the same tx
// as the line mutation that triggered it, after the caller has already
// locked the parent offer row via lockOfferForMutation.
func recomputeOfferTotals(ctx context.Context, tx *sql.Tx, offerID, workspaceID string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT quantity, unit_price_minor, discount_pct, tax_rate
		FROM offer_line_item
		WHERE offer_id=$1::uuid AND workspace_id=$2::uuid`,
		offerID, workspaceID)
	if err != nil {
		return fmt.Errorf("recompute totals query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var netMinor, taxMinor int64
	for rows.Next() {
		var quantity, discountPct, taxRate float64
		var unitPriceMinor int64
		if err := rows.Scan(&quantity, &unitPriceMinor, &discountPct, &taxRate); err != nil {
			return err
		}
		n, tx2 := computeLineTotals(quantity, unitPriceMinor, discountPct, taxRate)
		netMinor += n
		taxMinor += tx2
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE offer SET net_minor=$1, tax_minor=$2, gross_minor=$3
		WHERE id=$4::uuid AND workspace_id=$5::uuid`,
		netMinor, taxMinor, netMinor+taxMinor, offerID, workspaceID); err != nil {
		return fmt.Errorf("recompute totals update: %w", err)
	}
	return nil
}

// lockOfferForMutation SELECT ... FOR UPDATE-locks the parent offer row and
// returns its live status+version, or ErrNotFound if missing/archived. Every
// offer/line-item mutation calls this first, inside the same tx as its own
// write, so the draft-only check (OFFER-WIRE-4) is atomic against a
// concurrent status flip — never a separate read-then-write race.
func lockOfferForMutation(ctx context.Context, tx *sql.Tx, offerID, workspaceID string) (status string, version int64, err error) {
	err = tx.QueryRowContext(ctx, `
		SELECT status, version FROM offer
		WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL
		FOR UPDATE`,
		offerID, workspaceID).Scan(&status, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, errs.ErrNotFound
	}
	return status, version, err
}

// requireDraft returns ErrOfferNotDraft unless status is draft.
func requireDraft(status string) error {
	if status != domain.OfferStatusDraft {
		return ErrOfferNotDraft
	}
	return nil
}

// requireSent returns ErrOfferNotSent unless status is sent.
func requireSent(status string) error {
	if status != domain.OfferStatusSent {
		return ErrOfferNotSent
	}
	return nil
}

// loadWorkspaceNameAndBaseCurrency returns workspace.name and workspace.base_currency.
func loadWorkspaceNameAndBaseCurrency(ctx context.Context, tx *sql.Tx, workspaceID string) (name, baseCurrency string, err error) {
	err = tx.QueryRowContext(ctx, `
		SELECT name, base_currency FROM workspace WHERE id=$1::uuid`,
		workspaceID).Scan(&name, &baseCurrency)
	return name, baseCurrency, err
}

// buildBuyerSnapshot returns a stable buyer snapshot for send/render flows.
func (s *OfferStore) buildBuyerSnapshot(ctx context.Context, buyerOrgID *string, workspaceID string) (map[string]any, error) {
	if buyerOrgID == nil {
		return map[string]any{}, nil
	}
	org, err := organizations.NewOrgStore(s.db).Get(ctx, *buyerOrgID, workspaceID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"organization_id": org.ID,
		"display_name":    org.DisplayName,
		"address":         org.Address,
	}, nil
}

// OfferStore executes parameterized SQL against the offer table.
type OfferStore struct{ db *sql.DB }

// NewOfferStore returns an OfferStore backed by db.
func NewOfferStore(db *sql.DB) *OfferStore { return &OfferStore{db: db} }

func (s *OfferStore) checkOfferNumberConflict(ctx context.Context, tx *sql.Tx, workspaceID, offerNumber string, revision int64) error {
	var existingID string
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM offer WHERE workspace_id=$1::uuid AND offer_number=$2 AND revision=$3`,
		workspaceID, offerNumber, revision).Scan(&existingID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrDuplicateOfferNumber
}

// Create inserts a draft offer (status=draft, revision=1), its
// offer.created event_outbox row, and its create audit_log entry, in one
// workspace-scoped tx — after pre-checking the parent deal exists (404) and
// the offer_number+revision is unique (409, never a raw constraint error).
func (s *OfferStore) Create(ctx context.Context, o domain.Offer) (domain.Offer, error) {
	if err := sqlutil.RequireProvenance(o.Source, o.CapturedBy); err != nil {
		return domain.Offer{}, err
	}
	o.ID = ids.New()
	o.Status = domain.OfferStatusDraft
	o.Revision = 1
	err := database.WithWorkspaceTx(ctx, s.db, o.WorkspaceID, func(tx *sql.Tx) error {
		var dealExists bool
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM deal WHERE id=$1::uuid AND workspace_id=$2::uuid)`,
			o.DealID, o.WorkspaceID).Scan(&dealExists); err != nil {
			return err
		}
		if !dealExists {
			return errs.ErrNotFound
		}
		if err := s.checkOfferNumberConflict(ctx, tx, o.WorkspaceID, o.OfferNumber, o.Revision); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO offer (id, workspace_id, deal_id, offer_number, revision, status, currency,
			    buyer_org_id, valid_until, intro_text, terms_text, template_id, source, captured_by, version)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,1)`,
			o.ID, o.WorkspaceID, o.DealID, o.OfferNumber, o.Revision, o.Status, o.Currency,
			o.BuyerOrgID, o.ValidUntil, o.IntroText, o.TermsText, o.TemplateID, o.Source, o.CapturedBy); err != nil {
			return fmt.Errorf("offer create: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{"offer_id": o.ID, "deal_id": o.DealID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			o.WorkspaceID, "offer.created", o.ID, payload); err != nil {
			return fmt.Errorf("offer create event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOffer, &o.ID, nil, o)
		e.WorkspaceID = o.WorkspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer create audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, o.ID, o.WorkspaceID)
}

// The queries below spell out the full offer column list literally (rather
// than sharing it via a `+`-concatenated const) so SonarCloud's go:S2077
// rule — which traces a global identifier back through its own declaration —
// finds no concatenation to flag on any of these (see ProductStore's
// analogous comment in store_product.go).
const offerGetQuery = `SELECT
	id, workspace_id, deal_id, offer_number, revision, status, currency, buyer_org_id,
	buyer_snapshot, issuer_snapshot, valid_until, intro_text, terms_text,
	net_minor, tax_minor, gross_minor, fx_rate_to_base, fx_rate_date,
	template_id, pdf_asset_ref, accepted_at, version, source, captured_by,
	created_at, updated_at, archived_at
	FROM offer WHERE id=$1::uuid AND workspace_id=$2::uuid AND archived_at IS NULL`

// offerListQuery pushes the archived-filter into SQL itself via $5
// (includeArchived) rather than branching the query text in Go: $5=false
// yields "(false OR archived_at IS NULL)" (live rows only, same as before),
// $5=true short-circuits the OR to true (all rows, same as before) — one
// query instead of two near-identical literals.
const offerListQuery = `SELECT
	id, workspace_id, deal_id, offer_number, revision, status, currency, buyer_org_id,
	buyer_snapshot, issuer_snapshot, valid_until, intro_text, terms_text,
	net_minor, tax_minor, gross_minor, fx_rate_to_base, fx_rate_date,
	template_id, pdf_asset_ref, accepted_at, version, source, captured_by,
	created_at, updated_at, archived_at
	FROM offer WHERE workspace_id=$1::uuid AND deal_id=$2::uuid AND ($3 = '' OR id::text < $3)
	AND ($5 OR archived_at IS NULL)
	ORDER BY id DESC LIMIT $4`

func scanOffer(row interface{ Scan(dest ...any) error }) (domain.Offer, error) {
	var o domain.Offer
	var buyerSnap, issuerSnap []byte
	err := row.Scan(&o.ID, &o.WorkspaceID, &o.DealID, &o.OfferNumber, &o.Revision, &o.Status,
		&o.Currency, &o.BuyerOrgID, &buyerSnap, &issuerSnap, &o.ValidUntil, &o.IntroText,
		&o.TermsText, &o.NetMinor, &o.TaxMinor, &o.GrossMinor, &o.FxRateToBase, &o.FxRateDate,
		&o.TemplateID, &o.PdfAssetRef, &o.AcceptedAt, &o.Version, &o.Source, &o.CapturedBy,
		&o.CreatedAt, &o.UpdatedAt, &o.ArchivedAt)
	if err != nil {
		return o, err
	}
	sqlutil.UnmarshalJSON(buyerSnap, &o.BuyerSnapshot)
	sqlutil.UnmarshalJSON(issuerSnap, &o.IssuerSnapshot)
	return o, nil
}

// Get returns one live offer by id, workspace-scoped; ErrNotFound if absent
// or archived.
func (s *OfferStore) Get(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	var o domain.Offer
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, offerGetQuery, id, workspaceID)
		var scanErr error
		o, scanErr = scanOffer(row)
		return scanErr
	})
	if errors.Is(err, sql.ErrNoRows) {
		return o, errs.ErrNotFound
	}
	return o, err
}

// List returns dealID's offers, most-recent-first (descending id keyset —
// offer.id is uuidv7, time-ordered), cursor-paginated, honoring
// include_archived. 404s via the caller when dealID itself doesn't exist —
// List itself just returns an empty page for an unknown deal (mirrors
// ProductStore.List's "empty catalogue, not an error" precedent); the
// transport handler pre-checks deal existence for the documented 404.
func (s *OfferStore) List(ctx context.Context, workspaceID, dealID, cursor string, limit int, includeArchived bool) ([]domain.Offer, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	out := []domain.Offer{}
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, offerListQuery, workspaceID, dealID, cursor, limit+1, includeArchived)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			o, scanErr := scanOffer(rows)
			if scanErr != nil {
				return scanErr
			}
			out = append(out, o)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, "", err
	}
	var next string
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// nullDateOffer parses a "YYYY-MM-DD" string bounded-update value into a
// nullable SQL date param; nil for a missing/null/unparseable value. Note
// the caller's CASE WHEN hasKey(...) wrapper means an unparseable-but-present
// value clears the column to NULL rather than leaving it unchanged (hasKey
// is still true even though the parsed value is nil) — a documented, low-risk
// scope trade-off (see Global Constraint 11), distinct from the CREATE path,
// which validates in the transport handler and 400s instead, mirroring
// DealHandler.create's ExpectedCloseDate precedent.
func nullDateOffer(m map[string]any, key string) any {
	v, ok := m[key]
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return t
}

// Update applies a bounded partial update to currency/buyer_org_id/
// valid_until/intro_text/terms_text/template_id/source/captured_by, atomic
// with the OFFER-WIRE-4 draft-only guard and standard If-Match optimistic
// concurrency — both checked under the same row lock (lockOfferForMutation),
// never a separate read-then-write race. buyer_org_id/template_id need an
// explicit ::uuid cast on their CASE branch (a text param unified against a
// uuid column in a bare CASE errors "types uuid and text cannot be matched"
// — mirrors store_activity.go's `assignee_id = CASE WHEN $9 THEN $10::uuid
// ELSE assignee_id END`); valid_until already casts ::date for the same
// reason.
func (s *OfferStore) Update(ctx context.Context, id, workspaceID string, updates map[string]any, ifMatch int64) (domain.Offer, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, version, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if err := requireDraft(status); err != nil {
			return err
		}
		if ifMatch != 0 && ifMatch != version {
			return errs.ErrVersionSkew
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE offer SET
			  currency     = COALESCE($3, currency),
			  buyer_org_id = CASE WHEN $4 THEN $5::uuid ELSE buyer_org_id END,
			  valid_until  = CASE WHEN $6 THEN $7::date ELSE valid_until END,
			  intro_text   = CASE WHEN $8 THEN $9 ELSE intro_text END,
			  terms_text   = CASE WHEN $10 THEN $11 ELSE terms_text END,
			  template_id  = CASE WHEN $12 THEN $13::uuid ELSE template_id END,
			  source       = COALESCE($14, source),
			  captured_by  = COALESCE($15, captured_by)
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID,
			sqlutil.NullStr(updates, "currency"),
			hasKey(updates, "buyer_org_id"), sqlutil.NullStr(updates, "buyer_org_id"),
			hasKey(updates, "valid_until"), nullDateOffer(updates, "valid_until"),
			hasKey(updates, "intro_text"), sqlutil.NullStr(updates, "intro_text"),
			hasKey(updates, "terms_text"), sqlutil.NullStr(updates, "terms_text"),
			hasKey(updates, "template_id"), sqlutil.NullStr(updates, "template_id"),
			sqlutil.NullStr(updates, "source"),
			sqlutil.NullStr(updates, "captured_by")); err != nil {
			return fmt.Errorf("offer update: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOffer, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer update audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Send freezes the offer's FX metadata and buyer/issuer snapshots and marks it sent.
func (s *OfferStore) Send(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, _, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if err := requireDraft(status); err != nil {
			return err
		}

		var currency, dealID string
		var buyerOrgID sql.NullString
		if err := tx.QueryRowContext(ctx, `
			SELECT currency, deal_id, buyer_org_id FROM offer
			WHERE id=$1::uuid AND workspace_id=$2::uuid`,
			id, workspaceID).Scan(&currency, &dealID, &buyerOrgID); err != nil {
			return err
		}

		workspaceName, baseCurrency, err := loadWorkspaceNameAndBaseCurrency(ctx, tx, workspaceID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		rate := 1.0
		if currency != baseCurrency {
			rate, err = deals.AsOfFXRate(ctx, tx, workspaceID, currency, baseCurrency, now)
			if err != nil {
				return err
			}
		}

		var buyerOrgPtr *string
		if buyerOrgID.Valid {
			buyerOrgPtr = &buyerOrgID.String
		}
		buyerSnap, err := s.buildBuyerSnapshot(ctx, buyerOrgPtr, workspaceID)
		if err != nil {
			return err
		}

		buyerJSON, err := json.Marshal(buyerSnap)
		if err != nil {
			return err
		}
		issuerJSON, err := json.Marshal(map[string]any{"workspace_id": workspaceID, "name": workspaceName})
		if err != nil {
			return err
		}
		rateStr := strconv.FormatFloat(rate, 'f', 10, 64)

		if _, err := tx.ExecContext(ctx, `
			UPDATE offer SET status=$1, fx_rate_to_base=$2, fx_rate_date=$3,
			    buyer_snapshot=$4, issuer_snapshot=$5
			WHERE id=$6::uuid AND workspace_id=$7::uuid`,
			domain.OfferStatusSent, rateStr, now, buyerJSON, issuerJSON, id, workspaceID); err != nil {
			return fmt.Errorf("offer send: %w", err)
		}
		payload, _ := json.Marshal(map[string]any{"offer_id": id, "deal_id": dealID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload)
			 VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "offer.sent", id, payload); err != nil {
			return fmt.Errorf("offer send event: %w", err)
		}
		e := crmaudit.EntryFromPrincipal(ctx, "update", entityTypeOffer, &id, nil, nil)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer send audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, id, workspaceID)
}

// Regenerate clones a sent offer into a new draft revision and marks the
// prior revision superseded. The new revision keeps the prior line-item
// snapshot and provenance, but is inserted as a fresh row with a new id and
// revision+1.
func (s *OfferStore) Regenerate(ctx context.Context, id, workspaceID string) (domain.Offer, error) {
	var out domain.Offer
	err := database.WithWorkspaceTx(ctx, s.db, workspaceID, func(tx *sql.Tx) error {
		status, version, err := lockOfferForMutation(ctx, tx, id, workspaceID)
		if err != nil {
			return err
		}
		if err := requireSent(status); err != nil {
			return err
		}

		orig, err := scanOffer(tx.QueryRowContext(ctx, offerGetQuery, id, workspaceID))
		if err != nil {
			return err
		}

		out = orig
		out.ID = ids.New()
		out.Status = domain.OfferStatusDraft
		out.Revision = orig.Revision + 1
		out.Version = 1
		out.AcceptedAt = nil
		out.PdfAssetRef = nil
		out.CreatedAt = time.Time{}
		out.UpdatedAt = time.Time{}
		out.ArchivedAt = nil

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO offer (
				id, workspace_id, deal_id, offer_number, revision, status, currency,
				buyer_org_id, buyer_snapshot, issuer_snapshot, valid_until, intro_text,
				terms_text, net_minor, tax_minor, gross_minor, fx_rate_to_base,
				fx_rate_date, template_id, pdf_asset_ref, accepted_at, version,
				source, captured_by
			)
			SELECT
				$1::uuid, workspace_id, deal_id, offer_number, $2, $3, currency,
				buyer_org_id, buyer_snapshot, issuer_snapshot, valid_until, intro_text,
				terms_text, net_minor, tax_minor, gross_minor, fx_rate_to_base,
				fx_rate_date, template_id, NULL, NULL, 1,
				source, captured_by
			FROM offer
			WHERE id=$4::uuid AND workspace_id=$5::uuid`,
			out.ID, out.Revision, out.Status, id, workspaceID); err != nil {
			return fmt.Errorf("offer regenerate insert: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO offer_line_item (
				id, workspace_id, offer_id, position, product_id, description, unit,
				quantity, unit_price_minor, discount_pct, tax_rate
			)
			SELECT
				uuidv7(), workspace_id, $1::uuid, position, product_id, description, unit,
				quantity, unit_price_minor, discount_pct, tax_rate
			FROM offer_line_item
			WHERE offer_id=$2::uuid AND workspace_id=$3::uuid AND archived_at IS NULL`,
			out.ID, id, workspaceID); err != nil {
			return fmt.Errorf("offer regenerate lines: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE offer SET status=$2, version=$3
			WHERE id=$1::uuid AND workspace_id=$4::uuid`,
			id, domain.OfferStatusSuperseded, version+1, workspaceID); err != nil {
			return fmt.Errorf("offer regenerate supersede: %w", err)
		}

		payload, _ := json.Marshal(map[string]any{"offer_id": id, "deal_id": orig.DealID, "new_offer_id": out.ID})
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO event_outbox (workspace_id, topic, entity_id, payload) VALUES ($1,$2,$3::uuid,$4)`,
			workspaceID, "offer.superseded", out.ID, payload); err != nil {
			return fmt.Errorf("offer regenerate event: %w", err)
		}

		e := crmaudit.EntryFromPrincipal(ctx, "create", entityTypeOffer, &out.ID, nil, out)
		e.WorkspaceID = workspaceID
		if _, err := crmaudit.WriteTx(ctx, tx, e); err != nil {
			return fmt.Errorf("offer regenerate audit: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Offer{}, err
	}
	return s.Get(ctx, out.ID, workspaceID)
}
