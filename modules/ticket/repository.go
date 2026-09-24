package ticket

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go-starter-kit/modules/primitive"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) NextSeq(ctx context.Context) (int, error) {
	var seq int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var counter primitive.TicketCounter
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&counter, 1).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			counter = primitive.TicketCounter{ID: 1, Seq: 0, UpdatedAt: time.Now().UTC()}
			if err := tx.Create(&counter).Error; err != nil {
				return err
			}
		}
		counter.Seq++
		counter.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&counter).Error; err != nil {
			return err
		}
		seq = counter.Seq
		return nil
	})
	return seq, err
}

func (r *Repository) Save(ctx context.Context, rec Record) error {
	row := toModel(rec)
	var existing primitive.Ticket
	err := r.db.WithContext(ctx).Where("public_id = ?", rec.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	row.ID = existing.ID
	row.Seq = existing.Seq
	row.CreatedAt = existing.CreatedAt
	return r.db.WithContext(ctx).Save(&row).Error
}

func (r *Repository) FindByPublicID(ctx context.Context, publicID string) (Record, error) {
	var row primitive.Ticket
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return fromModel(row), nil
}

func (r *Repository) FindByThreadID(ctx context.Context, threadID string) (Record, error) {
	var row primitive.Ticket
	if err := r.db.WithContext(ctx).Where("thread_id = ?", threadID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	return fromModel(row), nil
}

func (r *Repository) ListOpen(ctx context.Context) ([]Record, error) {
	var rows []primitive.Ticket
	if err := r.db.WithContext(ctx).Where("status <> ?", "closed").Order("opened_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromModel(row))
	}
	return out, nil
}

func (r *Repository) ListByOpener(ctx context.Context, openerID string) ([]Record, error) {
	var rows []primitive.Ticket
	if err := r.db.WithContext(ctx).Where("opener_id = ?", openerID).Order("opened_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromModel(row))
	}
	return out, nil
}

func (r *Repository) ListOutgoingBySeller(ctx context.Context, sellerID string) ([]primitive.OutgoingMutation, error) {
	var rows []primitive.OutgoingMutation
	if err := r.db.WithContext(ctx).Where("seller_id = ?", sellerID).Order("paid_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) OutgoingExists(ctx context.Context, reference string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&primitive.OutgoingMutation{}).Where("reference = ?", reference).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SavePayment writes the ticket's new state and its outgoing ledger row in one
// transaction. Recording payment used to be two independent writes, so a
// failure between them (or a concurrent caller booking the same reference)
// could leave a ticket marked paid with no ledger row - the seller's record
// that the payout happened would simply be missing. The unique index on
// outgoing_mutations.reference still decides the winner; this only guarantees
// the ticket state and the ledger move together.
func (r *Repository) SavePayment(ctx context.Context, rec Record, outgoing primitive.OutgoingMutation) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := toModel(rec)
		var existing primitive.Ticket
		err := tx.Where("public_id = ?", rec.ID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			row.ID = existing.ID
			row.Seq = existing.Seq
			row.CreatedAt = existing.CreatedAt
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return tx.Create(&outgoing).Error
	})
}
func (r *Repository) Snapshot(ctx context.Context) (Snapshot, error) {
	var tickets []primitive.Ticket
	if err := r.db.WithContext(ctx).Select("status").Find(&tickets).Error; err != nil {
		return Snapshot{}, err
	}
	var payments []primitive.OutgoingMutation
	if err := r.db.WithContext(ctx).Select("currency", "amount_minor").Find(&payments).Error; err != nil {
		return Snapshot{}, err
	}
	out := Snapshot{Tickets: map[string]int{}, Totals: map[string]int64{}}
	for _, row := range tickets {
		out.Tickets[row.Status]++
		if row.Status != "closed" {
			out.OpenTickets++
		}
	}
	for _, row := range payments {
		out.Payments++
		out.Totals[row.Currency] += row.AmountMinor
	}
	return out, nil
}

// Optional jsonb columns are NULL when absent. They must NOT be written as an
// empty string: Postgres rejects ” as invalid JSON input, so every ticket
// insert without a resolution failed with 22P02.
func jsonOrNull(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func toModel(rec Record) primitive.Ticket {
	seq := 0
	if _, _, _, n, ok := ParseID(rec.ID); ok {
		seq = n
	}
	opened, _ := time.Parse(time.RFC3339Nano, rec.OpenedAt)
	if opened.IsZero() {
		opened, _ = time.Parse(time.RFC3339, rec.OpenedAt)
	}
	row := primitive.Ticket{
		PublicID: rec.ID, Seq: seq, Type: rec.Type, Code: rec.Code, Region: rec.Region,
		Workflow: rec.Workflow, Status: rec.Status, OpenerID: rec.OpenerID, OpenerTag: rec.OpenerTag,
		ThreadID: rec.ThreadID, AssignedTo: rec.AssignedTo, SummaryMsgID: rec.SummaryMsgID,
		OpenedAt: opened, DataJSON: EncodeJSON(rec.Data), HistoryJSON: EncodeJSON(rec.History),
	}
	if rec.ClosedAt != "" {
		t, err := time.Parse(time.RFC3339Nano, rec.ClosedAt)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, rec.ClosedAt)
		}
		row.ClosedAt = &t
	}
	if rec.Resolution != nil {
		row.ResolutionJSON = jsonOrNull(EncodeJSON(rec.Resolution))
	}
	if rec.Withdrawal != nil {
		row.WithdrawalJSON = jsonOrNull(EncodeJSON(rec.Withdrawal))
	}
	if rec.SellerApproval != nil {
		row.SellerJSON = jsonOrNull(EncodeJSON(rec.SellerApproval))
	}
	return row
}

func fromModel(row primitive.Ticket) Record {
	rec := Record{
		ID: row.PublicID, Type: row.Type, Code: row.Code, Region: row.Region, Workflow: row.Workflow,
		Status: row.Status, OpenerID: row.OpenerID, OpenerTag: row.OpenerTag, ThreadID: row.ThreadID,
		AssignedTo: row.AssignedTo, SummaryMsgID: row.SummaryMsgID, OpenedAt: row.OpenedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		Data: map[string]string{},
	}
	_ = json.Unmarshal([]byte(emptyJSON(row.DataJSON, "{}")), &rec.Data)
	_ = json.Unmarshal([]byte(emptyJSON(row.HistoryJSON, "[]")), &rec.History)
	if row.ResolutionJSON != nil && *row.ResolutionJSON != "" {
		var res Resolution
		if json.Unmarshal([]byte(*row.ResolutionJSON), &res) == nil {
			rec.Resolution = &res
		}
	}
	if row.WithdrawalJSON != nil && *row.WithdrawalJSON != "" {
		var wd Withdrawal
		if json.Unmarshal([]byte(*row.WithdrawalJSON), &wd) == nil {
			rec.Withdrawal = &wd
		}
	}
	if row.SellerJSON != nil && *row.SellerJSON != "" {
		var sa SellerApproval
		if json.Unmarshal([]byte(*row.SellerJSON), &sa) == nil {
			rec.SellerApproval = &sa
		}
	}
	if row.ClosedAt != nil {
		rec.ClosedAt = row.ClosedAt.UTC().Format("2006-01-02T15:04:05.000Z")
	}
	return rec
}

func emptyJSON(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
