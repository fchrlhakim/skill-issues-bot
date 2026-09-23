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

type PickerChoice struct {
	Key   string
	Role  string
	Label string
	Emoji string
}

var LanguageChoices = []PickerChoice{
	{Key: "en", Role: "English", Label: "English", Emoji: "🇬🇧"},
	{Key: "id", Role: "Bahasa Indonesia", Label: "Bahasa Indonesia", Emoji: "🇮🇩"},
	{Key: "ms", Role: "Bahasa Melayu", Label: "Bahasa Melayu", Emoji: "🇲🇾"},
	{Key: "fil", Role: "Filipino", Label: "Filipino", Emoji: "🇵🇭"},
	{Key: "th", Role: "Thai", Label: "ไทย", Emoji: "🇹🇭"},
	{Key: "vi", Role: "Vietnamese", Label: "Tiếng Việt", Emoji: "🇻🇳"},
	{Key: "hi", Role: "Hindi", Label: "हिन्दी", Emoji: "🇮🇳"},
	{Key: "bn", Role: "Bengali", Label: "বাংলা", Emoji: "🇧🇩"},
	{Key: "zh", Role: "Chinese", Label: "中文", Emoji: "🇨🇳"},
	{Key: "ja", Role: "Japanese", Label: "日本語", Emoji: "🇯🇵"},
	{Key: "ko", Role: "Korean", Label: "한국어", Emoji: "🇰🇷"},
}

var RegionChoices = []PickerChoice{
	{Key: "ID", Role: "Indonesia", Label: "Indonesia", Emoji: "🇮🇩"},
	{Key: "MY", Role: "Malaysia", Label: "Malaysia", Emoji: "🇲🇾"},
	{Key: "SG", Role: "Singapore", Label: "Singapore", Emoji: "🇸🇬"},
	{Key: "PH", Role: "Philippines", Label: "Philippines", Emoji: "🇵🇭"},
	{Key: "TH", Role: "Thailand", Label: "Thailand", Emoji: "🇹🇭"},
	{Key: "VN", Role: "Vietnam", Label: "Vietnam", Emoji: "🇻🇳"},
	{Key: "SA", Role: "South Asia", Label: "South Asia", Emoji: "🌏"},
	{Key: "CN", Role: "Greater China", Label: "Greater China", Emoji: "🌏"},
	{Key: "JP", Role: "Japan", Label: "Japan", Emoji: "🇯🇵"},
	{Key: "KR", Role: "South Korea", Label: "South Korea", Emoji: "🇰🇷"},
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
