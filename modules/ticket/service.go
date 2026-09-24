package ticket

import (
	"context"
	"encoding/json"
	"time"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/primitive"
)

type Actor struct {
	ID              string
	Username        string
	Tag             string
	Roles           []string
	IsAdmin         bool
	IsBot           bool
	OwnerID         string
	AvailableAdmins []string
}

type CreateInput struct {
	Type     string
	Data     map[string]string
	ThreadID string
}

type ServiceInterface interface {
	Create(ctx context.Context, actor Actor, input CreateInput) (Record, error)
	Get(ctx context.Context, publicID string) (Record, error)
	GetByThread(ctx context.Context, threadID string) (Record, error)
	ListOpen(ctx context.Context) ([]Record, error)
	Queue(ctx context.Context, actor Actor, page int) ([]Record, error)
	Claim(ctx context.Context, actor Actor, publicID string) (Record, error)
	Handoff(ctx context.Context, actor Actor, publicID, targetID, reason string) (Record, error)
	SetResolution(ctx context.Context, actor Actor, publicID, note string) (Record, error)
	Transition(ctx context.Context, actor Actor, publicID, to string) (Record, error)
	ApproveWithdrawal(ctx context.Context, actor Actor, publicID string) (Record, error)
	RecordPayment(ctx context.Context, actor Actor, publicID, reference string) (Record, error)
	ApproveSeller(ctx context.Context, actor Actor, publicID, reason string, openerRoles []string) (Record, error)
	Mutasi(ctx context.Context, actor Actor) ([]primitive.OutgoingMutation, error)
	Whoami(actor Actor) map[string]any
	Snapshot(ctx context.Context) (Snapshot, error)
}

type RepositoryInterface interface {
	NextSeq(ctx context.Context) (int, error)
	Save(ctx context.Context, rec Record) error
	FindByPublicID(ctx context.Context, publicID string) (Record, error)
	FindByThreadID(ctx context.Context, threadID string) (Record, error)
	ListOpen(ctx context.Context) ([]Record, error)
	ListByOpener(ctx context.Context, openerID string) ([]Record, error)
	ListOutgoingBySeller(ctx context.Context, sellerID string) ([]primitive.OutgoingMutation, error)
	// OutgoingExists reports whether a payment reference is already recorded for
	// ANY seller. A reference is a receipt, so it is globally single-use: without
	// this check the duplicate guard only sees the tickets the caller happened to
	// load, and the same receipt could be booked as two payouts.
	OutgoingExists(ctx context.Context, reference string) (bool, error)
	// SavePayment persists the ticket state and its outgoing ledger row
	// atomically, so a paid ticket can never exist without its ledger row.
	SavePayment(ctx context.Context, rec Record, outgoing primitive.OutgoingMutation) error
	Snapshot(ctx context.Context) (Snapshot, error)
}

type Service struct {
	repository RepositoryInterface
	now        func() time.Time
}

func NewService(repository RepositoryInterface) ServiceInterface {
	return &Service{repository: repository, now: time.Now}
}

type Snapshot struct {
	OpenTickets int
	Tickets     map[string]int
	Payments    int
	Totals      map[string]int64
}

func (s *Service) requireAdmin(actor Actor, rec Record) error {
	if actor.IsBot || !membership.IsTicketAdmin(actor.ID, actor.OwnerID, actor.IsBot, actor.IsAdmin) {
		return ErrNotAdmin
	}
	if rec.OpenerID == actor.ID {
		return ErrOpenerRecusal
	}
	return nil
}

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Record, error) {
	spec, ok := TypeByKey(input.Type)
	if !ok {
		return Record{}, ErrUnknownType
	}
	workflow := WorkflowForType(spec.Key)
	if workflow == "" {
		return Record{}, ErrUnknownWorkflow
	}
	if !membership.EligibleTicketApplicant(spec.Key, actor.Roles) {
		return Record{}, ErrNotEligible
	}
	data := map[string]string{}
	for key, raw := range input.Data {
		if raw == "" {
			continue
		}
		data[key] = NormalizeText(raw)
	}
	region := NormalizeText(data["region"])
	if !ValidRegion(region) {
		return Record{}, ErrInvalidRegion
	}
	data["region"] = region
	if spec.Key != "withdraw" {
		for _, key := range spec.Requires {
			if key == "region" {
				continue
			}
			if data[key] != "" {
				continue
			}
			for _, alias := range []string{"chronology", "summary", "notes", "amount", "product"} {
				if data[alias] != "" {
					data[key] = data[alias]
					break
				}
			}
			if data[key] == "" {
				return Record{}, ErrRequiredField
			}
		}
	}
	if len(SensitiveFindings(data)) > 0 {
		return Record{}, ErrSensitive
	}
	var wd *Withdrawal
	if spec.Key == "withdraw" {
		clean, built, err := ValidateWithdrawalInput(data)
		if err != nil {
			return Record{}, err
		}
		data = clean
		wd = &built
	}
	existing, err := s.repository.ListByOpener(ctx, actor.ID)
	if err != nil {
		return Record{}, err
	}
	if err := ActiveLimit(existing, actor.ID, spec.Key); err != nil {
		return Record{}, err
	}
	admins := 0
	for _, id := range actor.AvailableAdmins {
		if id != "" && id != actor.ID {
			admins++
		}
	}
	if admins == 0 {
		return Record{}, ErrNoAdmin
	}
	seq, err := s.repository.NextSeq(ctx)
	if err != nil {
		return Record{}, err
	}
	id := FormatID(spec.Code, data["region"], actor.Username, seq)
	threadID := input.ThreadID
	if threadID == "" {
		threadID = id
	}
	rec := NewRecord(id, spec.Key, spec.Code, data["region"], workflow, actor.ID, actor.Tag, threadID, data, wd, s.now())
	if err := s.repository.Save(ctx, rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

func (s *Service) Get(ctx context.Context, publicID string) (Record, error) {
	return s.repository.FindByPublicID(ctx, publicID)
}

func (s *Service) GetByThread(ctx context.Context, threadID string) (Record, error) {
	return s.repository.FindByThreadID(ctx, threadID)
}

func (s *Service) ListOpen(ctx context.Context) ([]Record, error) {
	return s.repository.ListOpen(ctx)
}

func (s *Service) Queue(ctx context.Context, actor Actor, page int) ([]Record, error) {
	if !membership.IsTicketAdmin(actor.ID, actor.OwnerID, actor.IsBot, actor.IsAdmin) {
		return nil, ErrNotAdmin
	}
	if page < 1 {
		page = 1
	}
	open, err := s.repository.ListOpen(ctx)
	if err != nil {
		return nil, err
	}
	const size = 10
	start := (page - 1) * size
	if start >= len(open) {
		return []Record{}, nil
	}
	end := start + size
	if end > len(open) {
		end = len(open)
	}
	return open[start:end], nil
}

func (s *Service) Claim(ctx context.Context, actor Actor, publicID string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	next, err := Claim(rec, actor.ID, s.now())
	if err != nil {
		return Record{}, err
	}
	if err := s.repository.Save(ctx, next); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) Handoff(ctx context.Context, actor Actor, publicID, targetID, reason string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	allowed := false
	for _, id := range actor.AvailableAdmins {
		if id == targetID && id != rec.OpenerID {
			allowed = true
			break
		}
	}
	if !allowed {
		return Record{}, ErrHandoffTarget
	}
	next, err := Handoff(rec, actor.ID, targetID, reason, s.now())
	if err != nil {
		return Record{}, err
	}
	if err := s.repository.Save(ctx, next); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) SetResolution(ctx context.Context, actor Actor, publicID, note string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	next, err := SetResolution(rec, actor.ID, note, s.now())
	if err != nil {
		return Record{}, err
	}
	if err := s.repository.Save(ctx, next); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) Transition(ctx context.Context, actor Actor, publicID, to string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	if to == "paid" && IsWithdrawal(rec.Type, rec.Workflow) {
		return Record{}, ErrIllegalTransition
	}
	next, err := Transition(rec, actor.ID, to, s.now())
	if err != nil {
		return Record{}, err
	}
	if err := s.repository.Save(ctx, next); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) ApproveWithdrawal(ctx context.Context, actor Actor, publicID string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	next, err := ApproveWithdrawal(rec, actor.ID, s.now().UTC().Format("2006-01-02T15:04:05.000Z"))
	if err != nil {
		return Record{}, err
	}
	if err := s.repository.Save(ctx, next); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) RecordPayment(ctx context.Context, actor Actor, publicID, reference string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	open, err := s.repository.ListOpen(ctx)
	if err != nil {
		return Record{}, err
	}
	byOpener, err := s.repository.ListByOpener(ctx, rec.OpenerID)
	if err != nil {
		return Record{}, err
	}
	others := append(open, byOpener...)
	at := s.now().UTC().Format("2006-01-02T15:04:05.000Z")
	next, err := RecordWithdrawalPayment(rec, actor.ID, at, reference, others)
	if err != nil {
		return Record{}, err
	}
	// The in-memory guard above only sees the tickets loaded for this call. The
	// durable table is the source of truth for "this receipt was already paid",
	// so it is consulted before anything is written.
	exists, err := s.repository.OutgoingExists(ctx, next.Withdrawal.Outgoing.Reference)
	if err != nil {
		return Record{}, err
	}
	if exists {
		return Record{}, ErrDuplicatePayment
	}
	if next.Withdrawal == nil || next.Withdrawal.Outgoing == nil {
		return Record{}, ErrWithdrawalInvalid
	}
	paidAt, err := time.Parse("2006-01-02T15:04:05.000Z", next.Withdrawal.Outgoing.At)
	if err != nil {
		return Record{}, err
	}
	// The outgoing ledger row is the seller's record that a payout happened, and
	// /revenue and /mutasi read it. Ticket state and ledger row are written in
	// one transaction: a paid ticket must never exist without the ledger row
	// that proves it, and the unique index on reference still decides which
	// concurrent caller wins.
	outgoing := primitive.OutgoingMutation{
		PublicID: next.Withdrawal.Outgoing.ID, TicketID: rec.ID, SellerID: rec.OpenerID,
		AmountMinor: next.Withdrawal.AmountMinor, Currency: next.Withdrawal.Currency,
		Reference: next.Withdrawal.Outgoing.Reference, By: actor.ID, PaidAt: paidAt,
	}
	if err := s.repository.SavePayment(ctx, next, outgoing); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) ApproveSeller(ctx context.Context, actor Actor, publicID, reason string, openerRoles []string) (Record, error) {
	rec, err := s.repository.FindByPublicID(ctx, publicID)
	if err != nil {
		return Record{}, err
	}
	if err := s.requireAdmin(actor, rec); err != nil {
		return Record{}, err
	}
	ok, _ := membership.DecideSellerApproval(openerRoles)
	if !ok {
		return Record{}, ErrNotEligible
	}
	next, err := ApproveSeller(rec, actor.ID, reason, rec.OpenerID, s.now())
	if err != nil {
		return Record{}, err
	}
	if err := s.repository.Save(ctx, next); err != nil {
		return Record{}, err
	}
	return next, nil
}

func (s *Service) Mutasi(ctx context.Context, actor Actor) ([]primitive.OutgoingMutation, error) {
	return s.repository.ListOutgoingBySeller(ctx, actor.ID)
}

func (s *Service) Whoami(actor Actor) map[string]any {
	return map[string]any{
		"id": actor.ID, "tier": membership.MemberTier(actor.Roles),
		"admin":      membership.IsTicketAdmin(actor.ID, actor.OwnerID, actor.IsBot, actor.IsAdmin),
		"restricted": membership.Restricted(actor.Roles),
	}
}
func (s *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	return s.repository.Snapshot(ctx)
}

func EncodeJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
