package membership

import "testing"

func TestDecideVerification(t *testing.T) {
	d := DecideVerification([]string{RoleUser}, 30, true, true)
	if d.Action != "verify" || d.AddRole != RoleBuyer || d.RemoveRole != RoleUser {
		t.Fatalf("%+v", d)
	}
	d = DecideVerification([]string{RoleUser}, 2, true, true)
	if !d.FlagYoungAccount {
		t.Fatal("young account")
	}
	d = DecideVerification([]string{"Suspended"}, 30, true, true)
	if d.Action != "refuse" {
		t.Fatal("restriction must win")
	}
	d = DecideVerification([]string{RoleSeller}, 30, true, true)
	if d.Action != "already" {
		t.Fatal("seller stays seller")
	}
}

func TestSellerApprovalRequiresBuyer(t *testing.T) {
	ok, reason := DecideSellerApproval([]string{RoleUser})
	if ok {
		t.Fatalf("user cannot be seller: %s", reason)
	}
	ok, _ = DecideSellerApproval([]string{RoleBuyer})
	if !ok {
		t.Fatal("buyer can be approved")
	}
}

func TestEligibleTicketApplicant(t *testing.T) {
	if EligibleTicketApplicant("withdraw", []string{RoleBuyer}) {
		t.Fatal("buyer cannot withdraw")
	}
	if !EligibleTicketApplicant("withdraw", []string{RoleSeller}) {
		t.Fatal("seller can withdraw")
	}
	if EligibleTicketApplicant("seller-verification", []string{RoleSeller}) {
		t.Fatal("seller cannot re-apply")
	}
	if !EligibleTicketApplicant("general", []string{RoleUser}) {
		t.Fatal("ungated types stay open")
	}
}

func TestResolvePickedRoleAllowList(t *testing.T) {
	role, add, ok := ResolvePickedRole("role:add:English")
	if !ok || !add || role != "English" {
		t.Fatalf("role=%s add=%v ok=%v", role, add, ok)
	}
	if _, _, ok := ResolvePickedRole("role:add:Platform Administrator"); ok {
		t.Fatal("staff role must be refused")
	}
}

func TestTicketAdmin(t *testing.T) {
	if !IsTicketAdmin("owner", "owner", false, false) {
		t.Fatal("owner")
	}
	if IsTicketAdmin("mod", "owner", false, false) {
		t.Fatal("role name is not authority")
	}
	if !IsTicketAdmin("mod", "owner", false, true) {
		t.Fatal("administrator bit")
	}
	if IsTicketAdmin("bot", "owner", true, true) {
		t.Fatal("bots cannot admin tickets")
	}
}
