package ticket

import (
	"context"
	"testing"
	"time"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/primitive"
)

type memRepo struct {
	seq      int
	tickets  map[string]Record
	outgoing []primitive.OutgoingMutation
	// failOutgoing simulates a ledger write that cannot be persisted.
	failOutgoing bool
}

func newMem() *memRepo {
	return &memRepo{tickets: map[string]Record{}}
}

func (m *memRepo) NextSeq(ctx context.Context) (int, error) {
	m.seq++
	return m.seq, nil
}
func (m *memRepo) Save(ctx context.Context, rec Record) error {
	m.tickets[rec.ID] = rec
	return nil
}
func (m *memRepo) FindByPublicID(ctx context.Context, publicID string) (Record, error) {
	rec, ok := m.tickets[publicID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return rec, nil
}
func (m *memRepo) FindByThreadID(ctx context.Context, threadID string) (Record, error) {
	for _, rec := range m.tickets {
		if rec.ThreadID == threadID {
			return rec, nil
		}
	}
	return Record{}, ErrNotFound
}
func (m *memRepo) ListOpen(ctx context.Context) ([]Record, error) {
	var out []Record
	for _, rec := range m.tickets {
		if !rec.Closed() {
			out = append(out, rec)
		}
	}
	return out, nil
}
func (m *memRepo) ListByOpener(ctx context.Context, openerID string) ([]Record, error) {
	var out []Record
	for _, rec := range m.tickets {
		if rec.OpenerID == openerID {
			out = append(out, rec)
		}
	}
	return out, nil
}
func (m *memRepo) ListOutgoingBySeller(ctx context.Context, sellerID string) ([]primitive.OutgoingMutation, error) {
	var out []primitive.OutgoingMutation
	for _, row := range m.outgoing {
		if row.SellerID == sellerID {
			out = append(out, row)
		}
	}
	return out, nil
}
func (m *memRepo) OutgoingExists(ctx context.Context, reference string) (bool, error) {
	for _, row := range m.outgoing {
		if row.Reference == reference {
			return true, nil
		}
	}
	return false, nil
}

func (m *memRepo) SavePayment(ctx context.Context, rec Record, outgoing primitive.OutgoingMutation) error {
	if m.failOutgoing {
		return ErrDuplicatePayment
	}
	m.tickets[rec.ID] = rec
	m.outgoing = append(m.outgoing, outgoing)
	return nil
}
func (m *memRepo) Snapshot(ctx context.Context) (Snapshot, error) {
	out := Snapshot{Tickets: map[string]int{}, Totals: map[string]int64{}}
	for _, rec := range m.tickets {
		out.Tickets[rec.Status]++
		if !rec.Closed() {
			out.OpenTickets++
		}
	}
	for _, row := range m.outgoing {
		out.Payments++
		out.Totals[row.Currency] += row.AmountMinor
	}
	return out, nil
}

func TestCreateSupportTicket(t *testing.T) {
	svc := NewService(newMem()).(*Service)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	rec, err := svc.Create(context.Background(), Actor{
		ID: "u1", Username: "alex", Roles: []string{membership.RoleBuyer},
		AvailableAdmins: []string{"admin1"},
	}, CreateInput{Type: "buyer-support", Data: map[string]string{"summary": "need help", "region": "ID"}})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Workflow != WorkflowSupport || rec.Status != "open" {
		t.Fatalf("%+v", rec)
	}
}

func TestWithdrawRequiresSeller(t *testing.T) {
	svc := NewService(newMem())
	_, err := svc.Create(context.Background(), Actor{
		ID: "u1", Username: "alex", Roles: []string{membership.RoleBuyer},
		AvailableAdmins: []string{"admin1"},
	}, CreateInput{Type: "withdraw", Data: map[string]string{
		"region": "ID", "transactionId": "SALE-1", "amount": "10.00", "currency": "USD", "method": "Bank",
	}})
	if err != ErrNotEligible {
		t.Fatalf("err=%v", err)
	}
}

func TestAdminCannotHandleOwnTicket(t *testing.T) {
	repo := newMem()
	svc := NewService(repo).(*Service)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	rec, err := svc.Create(context.Background(), Actor{
		ID: "admin1", Username: "boss", IsAdmin: true, OwnerID: "owner",
		Roles: []string{membership.RoleBuyer}, AvailableAdmins: []string{"admin2"},
	}, CreateInput{Type: "general", Data: map[string]string{"summary": "help", "region": "SG"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Claim(context.Background(), Actor{ID: "admin1", IsAdmin: true, OwnerID: "owner"}, rec.ID)
	if err != ErrOpenerRecusal {
		t.Fatalf("err=%v", err)
	}
}

func TestCloseNeedsResolution(t *testing.T) {
	repo := newMem()
	svc := NewService(repo).(*Service)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	rec, err := svc.Create(context.Background(), Actor{
		ID: "u1", Username: "alex", Roles: []string{membership.RoleBuyer}, AvailableAdmins: []string{"admin1"},
	}, CreateInput{Type: "general", Data: map[string]string{"summary": "help", "region": "SG"}})
	if err != nil {
		t.Fatal(err)
	}
	admin := Actor{ID: "admin1", IsAdmin: true, OwnerID: "owner"}
	rec, err = svc.Transition(context.Background(), admin, rec.ID, "in-progress")
	if err != nil {
		t.Fatal(err)
	}
	rec, err = svc.Transition(context.Background(), admin, rec.ID, "resolved")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Transition(context.Background(), admin, rec.ID, "closed"); err != ErrResolutionMissing {
		t.Fatalf("err=%v", err)
	}
	if _, err := svc.SetResolution(context.Background(), admin, rec.ID, "done, member notified"); err != nil {
		t.Fatal(err)
	}
	closed, err := svc.Transition(context.Background(), admin, rec.ID, "closed")
	if err != nil || closed.Status != "closed" {
		t.Fatalf("err=%v status=%s", err, closed.Status)
	}
}

// withdrawalFixture opens and approves a withdrawal so a payment can be recorded
// against it.
func withdrawalFixture(t *testing.T, repo *memRepo, svc *Service, sellerID, reference string) Record {
	t.Helper()
	ctx := context.Background()
	rec, err := svc.Create(ctx, Actor{
		ID: sellerID, Username: "seller", Roles: []string{membership.RoleSeller},
		AvailableAdmins: []string{"admin1"},
	}, CreateInput{Type: "withdraw", Data: map[string]string{
		"region": "ID", "transactionId": reference, "amount": "10.00", "currency": "USD", "method": "Bank",
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	admin := Actor{ID: "admin1", IsAdmin: true, OwnerID: "owner"}
	if _, err := svc.ApproveWithdrawal(ctx, admin, rec.ID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return rec
}

// A payment reference is a receipt: it proves one real-world payout. Booking it
// twice would pay a seller twice for one receipt, and the guard used to only see
// the tickets loaded for the current call, so a second withdrawal from a
// different seller could reuse it.
func TestDuplicatePaymentReferenceIsRejected(t *testing.T) {
	repo := newMem()
	svc := NewService(repo).(*Service)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	admin := Actor{ID: "admin1", IsAdmin: true, OwnerID: "owner"}

	first := withdrawalFixture(t, repo, svc, "seller-1", "SALE-1")
	if _, err := svc.RecordPayment(ctx, admin, first.ID, "RECEIPT-1"); err != nil {
		t.Fatalf("first payment: %v", err)
	}
	// Close it, which is the real end state of a paid withdrawal. A closed ticket
	// drops out of ListOpen, so the in-memory guard in RecordPayment can no longer
	// see this receipt at all — only the durable check can.
	if _, err := svc.SetResolution(ctx, admin, first.ID, "paid out, receipt RECEIPT-1"); err != nil {
		t.Fatalf("resolution: %v", err)
	}
	if _, err := svc.Transition(ctx, admin, first.ID, "closed"); err != nil {
		t.Fatalf("close first ticket: %v", err)
	}

	// A DIFFERENT seller reuses the same receipt.
	second := withdrawalFixture(t, repo, svc, "seller-2", "SALE-2")
	if _, err := svc.RecordPayment(ctx, admin, second.ID, "RECEIPT-1"); err != ErrDuplicatePayment {
		t.Fatalf("err=%v, want ErrDuplicatePayment", err)
	}
	if len(repo.outgoing) != 1 {
		t.Fatalf("outgoing ledger has %d rows, want 1", len(repo.outgoing))
	}
}

// The outgoing ledger row is what /revenue and /mutasi report. A failure to
// record it must surface, not be swallowed while the caller reports success.
func TestOutgoingLedgerFailureIsReported(t *testing.T) {
	repo := newMem()
	repo.failOutgoing = true
	svc := NewService(repo).(*Service)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	rec := withdrawalFixture(t, repo, svc, "seller-1", "SALE-1")

	if _, err := svc.RecordPayment(context.Background(), Actor{ID: "admin1", IsAdmin: true, OwnerID: "owner"}, rec.ID, "RECEIPT-1"); err == nil {
		t.Fatal("a failed ledger write was reported as success")
	}
}

// A receipt that already exists must not be recorded, and the check must consult
// the durable store rather than the caller's loaded tickets.
func TestOutgoingExistsSeesOtherSellers(t *testing.T) {
	repo := newMem()
	svc := NewService(repo).(*Service)
	svc.now = func() time.Time { return time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC) }
	ctx := context.Background()

	rec := withdrawalFixture(t, repo, svc, "seller-1", "SALE-1")
	if _, err := svc.RecordPayment(ctx, Actor{ID: "admin1", IsAdmin: true, OwnerID: "owner"}, rec.ID, "RECEIPT-9"); err != nil {
		t.Fatal(err)
	}
	found, err := repo.OutgoingExists(ctx, "RECEIPT-9")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	missing, err := repo.OutgoingExists(ctx, "RECEIPT-NEVER")
	if err != nil || missing {
		t.Fatalf("found=%v err=%v for an unused reference", missing, err)
	}
}
