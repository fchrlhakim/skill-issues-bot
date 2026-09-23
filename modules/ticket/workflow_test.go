package ticket

import "testing"

func TestCanTransitionSupportAndWithdrawal(t *testing.T) {
	if !CanTransition("buyer-support", WorkflowSupport, "open", "in-progress") {
		t.Fatal("support open -> in-progress")
	}
	if CanTransition("buyer-support", WorkflowSupport, "open", "closed") {
		t.Fatal("support must not jump to closed")
	}
	if CanTransition("withdraw", WorkflowWithdrawal, "approved", "paid") {
		t.Fatal("paid is not a generic edge")
	}
	if !CanTransition("withdraw", WorkflowWithdrawal, "open", "approved") {
		t.Fatal("withdrawal open -> approved")
	}
	if WorkflowForType("withdraw") != WorkflowWithdrawal {
		t.Fatal("withdraw workflow")
	}
	if _, ok := WorkflowForTicket("withdraw", "nope", "open"); ok {
		t.Fatal("mismatched workflow must fail closed")
	}
}

func TestTicketIDRoundTrip(t *testing.T) {
	id := FormatID("BUY", "ID", "Alex!!", 125)
	if id != "BUY-ID-alex-00125" {
		t.Fatalf("id=%s", id)
	}
	code, region, user, seq, ok := ParseID(id)
	if !ok || code != "BUY" || region != "ID" || user != "alex" || seq != 125 {
		t.Fatalf("parse %s %s %s %d", code, region, user, seq)
	}
}

func TestValidateTicketTextRejectsSecrets(t *testing.T) {
	if _, err := ValidateTicketText("password is hunter2"); err != ErrSensitive {
		t.Fatalf("err=%v", err)
	}
	if _, err := ValidateTicketText("   "); err != ErrTextEmpty {
		t.Fatalf("empty err=%v", err)
	}
	got, err := ValidateTicketText("need help with a late delivery")
	if err != nil || got != "need help with a late delivery" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestSensitiveCardNumber(t *testing.T) {
	findings := SensitiveFindings(map[string]string{"text": "card 4111111111111111"})
	if len(findings) == 0 {
		t.Fatal("expected PAN finding")
	}
	if len(SensitiveFindings(map[string]string{"text": "user id 123456789012345678"})) != 0 {
		t.Fatal("discord-like digits should not always trip Luhn")
	}
}
