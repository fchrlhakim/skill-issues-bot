package membership

import "strings"

const YoungAccountDays = 7

const VerifyButtonID = "verify:accept"

type VerifyDecision struct {
	Action           string
	Reason           string
	Audit            string
	AddRole          string
	RemoveRole       string
	FlagYoungAccount bool
}

func DecideVerification(roleNames []string, accountAgeDays int, verifiedExists, verifiedAssignable bool) VerifyDecision {
	if roleNames == nil {
		return VerifyDecision{Action: "refuse", Reason: "Your member roles need an admin review.", Audit: "verify.refused_roles"}
	}
	has := func(n string) bool {
		for _, name := range roleNames {
			if name == n {
				return true
			}
		}
		return false
	}
	for _, blocked := range []string{"Suspended", "Blacklisted"} {
		if has(blocked) {
			return VerifyDecision{
				Action: "refuse",
				Reason: "Your account is currently **" + blocked + "**. Marketplace access is unavailable until that is resolved. Open a ticket if you believe this is a mistake.",
				Audit:  "verify.refused_restricted",
			}
		}
	}
	if has("Under Review") {
		return VerifyDecision{
			Action: "refuse",
			Reason: "Your account is **Under Review**. An admin is looking at it; please respond in your existing ticket rather than re-verifying.",
			Audit:  "verify.refused_under_review",
		}
	}
	tier := MemberTier(roleNames)
	if tier == TierConflict {
		return VerifyDecision{Action: "refuse", Reason: "Your member roles need an admin review before verification.", Audit: "verify.refused_roles"}
	}
	if has(RoleBuyer) || has(RoleSeller) {
		return VerifyDecision{Action: "already", Reason: "You already have marketplace access. Your current member tier is unchanged."}
	}
	if !verifiedExists {
		return VerifyDecision{Action: "error", Reason: "Marketplace access is not configured yet. Please tell an admin.", Audit: "verify.misconfigured_missing_role"}
	}
	if !verifiedAssignable {
		return VerifyDecision{Action: "error", Reason: "Marketplace access cannot complete because of a role-hierarchy problem. Please contact an admin.", Audit: "verify.misconfigured_hierarchy"}
	}
	remove := ""
	if has(RoleUser) {
		remove = RoleUser
	}
	return VerifyDecision{
		Action:           "verify",
		AddRole:          RoleBuyer,
		RemoveRole:       remove,
		FlagYoungAccount: accountAgeDays < YoungAccountDays,
		Audit:            "verify.granted",
	}
}

func DecideSellerApproval(roleNames []string) (ok bool, reason string) {
	if Restricted(roleNames) {
		return false, "Membership is restricted. An admin must review it before promotion."
	}
	switch MemberTier(roleNames) {
	case TierSeller:
		return true, "already"
	case TierBuyer:
		return true, ""
	case TierConflict:
		return false, "Conflicting member tiers need reconciliation before promotion."
	default:
		return false, "Seller approval requires a current Buyer. Verify first."
	}
}

func IsTicketAdmin(memberID, ownerID string, isBot, hasAdministrator bool) bool {
	if memberID == "" || isBot {
		return false
	}
	return memberID == ownerID || hasAdministrator
}

func ResolvePickedRole(customID string) (role string, ok bool) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 || parts[0] != "rolepick" {
		return "", false
	}
	switch parts[1] {
	case "lang":
		for _, c := range LanguageChoices {
			if c.Key == parts[2] {
				role = c.Role
			}
		}
	case "region":
		for _, c := range RegionChoices {
			if c.Key == parts[2] {
				role = c.Role
			}
		}
	default:
		return "", false
	}
	if role == "" || !IsPickable(role) || IsNeverSelfAssignable(role) {
		return "", false
	}
	return role, true
}
