package membership

const (
	TierUser     = "user"
	TierBuyer    = "buyer"
	TierSeller   = "seller"
	TierNone     = "none"
	TierConflict = "conflict"

	RoleUser   = "User"
	RoleBuyer  = "Buyer"
	RoleSeller = "Seller"
)

var MemberRoles = map[string]string{
	TierUser:   RoleUser,
	TierBuyer:  RoleBuyer,
	TierSeller: RoleSeller,
}

var MemberRoleNames = []string{RoleUser, RoleBuyer, RoleSeller}

var RestrictionRoles = []string{"Under Review", "Suspended", "Blacklisted"}

var LegacyMemberRoles = []string{
	"New Member", "Verified Seller", "Provisional Seller", "Verified Buyer",
}

var PickableRoles = []string{
	"English", "Bahasa Indonesia", "Bahasa Melayu", "Filipino", "Thai",
	"Vietnamese", "Hindi", "Bengali", "Chinese", "Japanese", "Korean",
	"Indonesia", "Malaysia", "Singapore", "Philippines", "Thailand",
	"Vietnam", "South Asia", "Greater China", "Japan", "South Korea",
}

var NeverSelfAssignable = []string{
	"Server Owner", "Platform Administrator", "Operations Lead",
	"Intermediary Lead", "Intermediary", "Dispute & Compliance",
	"Security & Fraud", "Finance & Risk", "Support & Verification",
	"Regional & Translation", "Moderator", "Developer",
	RoleUser, RoleBuyer, RoleSeller,
	"Verified Seller", "Provisional Seller", "Verified Buyer",
	"Under Review", "Suspended", "Blacklisted",
}

func MemberTier(roleNames []string) string {
	if roleNames == nil {
		return TierConflict
	}
	var tiers []string
	for _, name := range roleNames {
		for _, legacy := range LegacyMemberRoles {
			if name == legacy {
				return TierConflict
			}
		}
		for key, label := range MemberRoles {
			if name == label {
				tiers = append(tiers, key)
			}
		}
	}
	if len(tiers) > 1 {
		return TierConflict
	}
	if len(tiers) == 1 {
		return tiers[0]
	}
	return TierNone
}

func Restricted(roleNames []string) bool {
	for _, name := range roleNames {
		for _, blocked := range RestrictionRoles {
			if name == blocked {
				return true
			}
		}
	}
	return false
}

func IsPickable(role string) bool {
	for _, name := range PickableRoles {
		if name == role {
			return true
		}
	}
	return false
}

func IsNeverSelfAssignable(role string) bool {
	for _, name := range NeverSelfAssignable {
		if name == role {
			return true
		}
	}
	return false
}

func EligibleTicketApplicant(ticketType string, roleNames []string) bool {
	gated := ticketType == "withdraw" || ticketType == "seller-verification" || ticketType == "campaign" || ticketType == "affiliate"
	if !gated {
		return true
	}
	if Restricted(roleNames) {
		return false
	}
	tier := MemberTier(roleNames)
	if ticketType == "withdraw" {
		return tier == TierSeller
	}
	if ticketType == "seller-verification" {
		return tier == TierBuyer
	}
	return tier == TierBuyer || tier == TierSeller
}

func ExclusiveGrant(current []string, target string) (add string, remove []string, ok bool) {
	label, exists := MemberRoles[target]
	if !exists {
		return "", nil, false
	}
	for key, name := range MemberRoles {
		if key == target {
			continue
		}
		for _, have := range current {
			if have == name {
				remove = append(remove, name)
			}
		}
	}
	return label, remove, true
}
