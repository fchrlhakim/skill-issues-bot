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
func (m *memRepo) SaveOutgoing(ctx context.Context, rec primitive.OutgoingMutation) error {
	m.outgoing = append(m.outgoing, rec)
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
