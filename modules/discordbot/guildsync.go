package discordbot

import "github.com/bwmarrin/discordgo"

type channelSpec struct {
	Key   string
	Topic string
	Voice bool
}

type categorySpec struct {
	Key      string
	Display  string
	Channels []channelSpec
}

func launchLayout() []categorySpec {
	return []categorySpec{
		{Key: "start-here", Display: "🚀 ~ Start Here", Channels: []channelSpec{
			{Key: "welcome", Topic: "What Skillissue.ai is, and how to begin. Read #rules next."},
			{Key: "goodbye", Topic: "Member departure notices. Minimal public information."},
			{Key: "rules", Topic: "Server and marketplace rules."},
			{Key: "how-it-works", Topic: "Buyer flow, seller flow, tickets."},
			{Key: "verification", Topic: "User → Buyer through Verify. Seller needs admin review."},
			{Key: "faq", Topic: "Common questions."},
			{Key: "service-status", Topic: "Bot, ticket, and service status."},
			{Key: "security-notices", Topic: "Scam patterns and safety alerts."},
			{Key: "roles-and-languages", Topic: "Pick language and region for routing."},
		}},
		{Key: "transaction-support", Display: "🎫 ~ Transaction Support", Channels: []channelSpec{
			{Key: "open-a-ticket", Topic: "Private admin-handled tickets."},
			{Key: "discount-campaign", Topic: "Campaign information. No automatic discounts."},
			{Key: "affiliate", Topic: "Affiliate information. No automatic commissions."},
			{Key: "transaction-guidelines", Topic: "Transaction procedure and checklists."},
			{Key: "payment-guidelines", Topic: "Payment practice and scam warnings."},
			{Key: "delivery-confirmation", Topic: "How delivery is confirmed."},
			{Key: "refund-and-cancellation", Topic: "Refund and cancellation policy."},
			{Key: "disputes", Topic: "Dispute procedure and evidence."},
			{Key: "ticket-archive", Topic: "Closed-ticket guidance."},
		}},
		{Key: "seller-area", Display: "🏪 ~ Seller Area", Channels: []channelSpec{
			{Key: "seller-general", Topic: "Seller conversation."},
			{Key: "seller-application", Topic: "Apply via Seller Verification ticket."},
			{Key: "seller-guidelines", Topic: "Seller rules and listing standards."},
			{Key: "seller-resources", Topic: "Templates and anti-scam guide."},
			{Key: "seller-support", Topic: "Seller help."},
			{Key: "seller-updates", Topic: "Seller policy notices."},
			{Key: "seller-withdraw", Topic: "Withdrawal requests: admin review, then /withdraw-paid."},
		}},
		{Key: "buyer-area", Display: "🛍️ ~ Buyer Area", Channels: []channelSpec{
			{Key: "buyer-general", Topic: "Buyer conversation."},
			{Key: "buyer-help", Topic: "Buyer help."},
			{Key: "buyer-guidelines", Topic: "Buyer rules."},
			{Key: "buyer-updates", Topic: "Buyer notices."},
		}},
		{Key: "languages", Display: "🌐 ~ Language Areas", Channels: []channelSpec{
			{Key: "english", Topic: "Primary cross-border channel. English."},
			{Key: "bahasa-indonesia", Topic: "Diskusi Bahasa Indonesia."},
		}},
		{Key: "internal", Display: "🛡️ ~ Internal Operations", Channels: []channelSpec{
			{Key: "staff-log", Topic: "Staff action log."},
			{Key: "audit-log", Topic: "Audit trail."},
		}},
	}
}

func displayChannel(key string) string {
	emoji := map[string]string{
		"welcome": "👋", "goodbye": "👋", "rules": "📜", "how-it-works": "🧭", "verification": "✅",
		"faq": "❓", "service-status": "🟢", "security-notices": "🚨", "roles-and-languages": "🌐",
		"open-a-ticket": "🎫", "discount-campaign": "🎯", "affiliate": "🤝",
		"transaction-guidelines": "📋", "payment-guidelines": "💳", "delivery-confirmation": "📬",
		"refund-and-cancellation": "↩️", "disputes": "⚖️", "ticket-archive": "🗄️",
		"seller-general": "💬", "seller-application": "📝", "seller-guidelines": "📘",
		"seller-resources": "🧰", "seller-support": "🆘", "seller-updates": "📣", "seller-withdraw": "💸",
		"buyer-general": "💬", "buyer-help": "💡", "buyer-guidelines": "📗", "buyer-updates": "📣",
		"english": "🇬🇧", "bahasa-indonesia": "🇮🇩", "staff-log": "📋", "audit-log": "📑",
	}
	mark := emoji[key]
	if mark == "" {
		mark = "💬"
	}
	return mark + "｜" + key
}

func channelMatches(name, key string) bool {
	return name == key || name == displayChannel(key)
}

func (b *Bot) syncLaunchGuild(s *discordgo.Session) (cats, chans int, err error) {
	existing, err := s.GuildChannels(b.guildID)
	if err != nil {
		return 0, 0, err
	}
	catID := map[string]string{}
	have := map[string]bool{}
	for _, ch := range existing {
		if ch.Type == discordgo.ChannelTypeGuildCategory {
			for _, spec := range launchLayout() {
				if ch.Name == spec.Display || ch.Name == spec.Key {
					catID[spec.Key] = ch.ID
				}
			}
			continue
		}
		for _, spec := range launchLayout() {
			for _, c := range spec.Channels {
				if channelMatches(ch.Name, c.Key) {
					have[c.Key] = true
				}
			}
		}
	}
	for _, spec := range launchLayout() {
		parent := catID[spec.Key]
		if parent == "" {
			created, err := s.GuildChannelCreateComplex(b.guildID, discordgo.GuildChannelCreateData{
				Name: spec.Display, Type: discordgo.ChannelTypeGuildCategory,
			})
			if err != nil {
				return cats, chans, err
			}
			parent = created.ID
			catID[spec.Key] = parent
			cats++
		}
		for _, c := range spec.Channels {
			if have[c.Key] {
				continue
			}
			kind := discordgo.ChannelTypeGuildText
			if c.Voice {
				kind = discordgo.ChannelTypeGuildVoice
			}
			if _, err := s.GuildChannelCreateComplex(b.guildID, discordgo.GuildChannelCreateData{
				Name: displayChannel(c.Key), Type: kind, Topic: c.Topic, ParentID: parent,
			}); err != nil {
				return cats, chans, err
			}
			have[c.Key] = true
			chans++
		}
	}
	b.refreshChannels()
	return cats, chans, nil
}
