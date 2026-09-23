package ticket

import (
	"testing"
	"time"
)

func TestParseWithdrawalAmount(t *testing.T) {
	minor, cur, display, err := ParseAmount("12.50", "usd")
	if err != nil || minor != 1250 || cur != "USD" || display != "12.50" {
		t.Fatalf("minor=%d cur=%s display=%s err=%v", minor, cur, display, err)
	}
	if _, _, _, err := ParseAmount("12.5", "JPY"); err == nil {
		t.Fatal("JPY must be whole units")
	}
}

func TestValidatePaymentReferenceRejectsWallet(t *testing.T) {
	if _, err := ValidatePaymentReference("0xabcdeffedcbaabcdeffedcbaabcdeffedcbaabcd"); err == nil {
		t.Fatal("eth address")
	}
	ref, err := ValidatePaymentReference("INV-2026.01")
	if err != nil || ref != "INV-2026.01" {
		t.Fatalf("ref=%s err=%v", ref, err)
	}
}

func TestApproveAndPayWithdrawal(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	data, wd, err := ValidateWithdrawalInput(map[string]string{
		"region": "ID", "transactionId": "SALE-99", "amount": "10.00", "currency": "USD", "method": "Bank transfer",
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := NewRecord("WD-ID-seller-00001", "withdraw", "WD", "ID", WorkflowWithdrawal, "seller1", "", "thread1", data, &wd, now)
	approved, err := ApproveWithdrawal(rec, "admin1", now.Add(time.Minute).UTC().Format("2006-01-02T15:04:05.000Z"))
	if err != nil || approved.Status != "approved" {
		t.Fatalf("approve err=%v status=%s", err, approved.Status)
	}
	paid, err := RecordWithdrawalPayment(approved, "admin1", now.Add(2*time.Minute).UTC().Format("2006-01-02T15:04:05.000Z"), "RCPT-1", []Record{approved})
	if err != nil || paid.Status != "paid" || paid.Withdrawal.Outgoing == nil {
		t.Fatalf("pay err=%v status=%s", err, paid.Status)
	}
	if _, err := RecordWithdrawalPayment(paid, "admin1", now.Add(3*time.Minute).UTC().Format("2006-01-02T15:04:05.000Z"), "RCPT-2", []Record{paid}); err != ErrAlreadyPaid {
		t.Fatalf("second pay err=%v", err)
	}
}

func TestActiveTicketLimit(t *testing.T) {
	open := []Record{
		{OpenerID: "m1", Type: "general", Status: "open"},
		{OpenerID: "m1", Type: "regional", Status: "open"},
		{OpenerID: "m1", Type: "translation", Status: "in-progress"},
	}
	if err := ActiveLimit(open, "m1", "buyer-support"); err != ErrOpenLimit {
		t.Fatalf("err=%v", err)
	}
	if err := ActiveLimit(open[:1], "m1", "general"); err != ErrDuplicateType {
		t.Fatalf("err=%v", err)
	}
}
