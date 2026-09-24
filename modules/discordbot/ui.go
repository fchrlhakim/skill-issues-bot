package discordbot

import (
	"fmt"
	"strings"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/ticket"

	"github.com/bwmarrin/discordgo"
)

const disclaimer = "Skillissue.ai coordinates and reviews. It does not hold funds or guarantee outcomes."

func ticketPanel() *discordgo.InteractionResponseData {
	opts := make([]discordgo.SelectMenuOption, 0, len(ticket.Types))
	for _, t := range ticket.Types {
		desc := t.Description
		if len(desc) > 100 {
			desc = desc[:100]
		}
		opts = append(opts, discordgo.SelectMenuOption{Label: t.Label, Value: t.Key, Description: desc, Emoji: &discordgo.ComponentEmoji{Name: t.Emoji}})
	}
	return &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{{
			Title: "Open a Ticket — Skillissue.ai", Color: 0x4A2FBD, Footer: &discordgo.MessageEmbedFooter{Text: disclaimer},
			Description: "Members can open a ticket using the menu below.\n**Only server admins handle tickets.**\nYou can have one active ticket per type, up to three active tickets in total.\n\n**Never share** passwords, OTPs, PINs, CVVs, full card numbers, API or private keys, or seed phrases.",
		}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "ticket:open", Placeholder: "Choose the type of help you need", Options: opts},
			}},
		},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func ticketModal(t ticket.Type) *discordgo.InteractionResponseData {
	short := discordgo.TextInputShort
	para := discordgo.TextInputParagraph
	fields := []discordgo.MessageComponent{
		textInput("region", "Country / region code", "e.g. ID, MY, SG; XX for other", short, true, 4),
	}
	switch t.Key {
	case "withdraw":
		fields = append(fields,
			textInput("transactionId", "Sale reference", "e.g. SALE-2026-001; no account details", short, true, ticket.PaymentRefLimit),
			textInput("amount", "Exact withdrawal amount", "e.g. 1250.50", short, true, 40),
			textInput("currency", "ISO currency code", "e.g. IDR, USD, SGD", short, true, 3),
			textInput("method", "Payout method label only", "e.g. Bank transfer", short, true, ticket.MethodLimit),
		)
	case "purchase", "sale", "introduction":
		fields = append(fields,
			textInput("product", "Product or service", "Product name and listing reference", short, true, 300),
			textInput("amount", "Quantity, price and currency", "e.g. 1 unit at 1500000 IDR", short, true, 300),
			textInput("methods", "Payment and delivery methods", "e.g. bank transfer; delivery in 2 hours", short, true, 300),
			textInput("notes", "Terms and additional details", "No credentials", para, false, 900),
		)
	case "dispute", "refund", "scam", "impersonation", "delivery":
		needID := t.Key != "scam" && t.Key != "impersonation"
		fields = append(fields,
			textInput("transactionId", "Transaction or ticket ID", "e.g. BUY-ID-alex-00125", short, needID, 300),
			textInput("amount", "Amount and currency (if applicable)", "leave blank if none", short, false, 300),
			textInput("chronology", "What happened and what help is needed?", "Dates YYYY-MM-DD, timezone", para, true, 900),
			textInput("evidence", "Available evidence (redacted only)", "Remove credentials", para, false, 500),
		)
	default:
		fields = append(fields,
			textInput("language", "Preferred language (optional)", "e.g. English", short, false, 80),
			textInput("chronology", "How can an admin help?", "Summarise the issue; no sensitive data", para, true, 900),
		)
	}
	title := t.Label
	if len(title) > 45 {
		title = title[:45]
	}
	components := make([]discordgo.MessageComponent, 0, len(fields))
	for _, f := range fields {
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{f}})
	}
	return &discordgo.InteractionResponseData{
		CustomID:   "ticket:modal:" + t.Key,
		Title:      title,
		Components: components,
	}
}

func textInput(id, label, placeholder string, style discordgo.TextInputStyle, required bool, max int) discordgo.TextInput {
	return discordgo.TextInput{CustomID: id, Label: label, Placeholder: placeholder, Style: style, Required: required, MaxLength: max}
}

func ticketControls(rec ticket.Record) []discordgo.MessageComponent {
	_, ok := ticket.StatusFor(rec.Type, rec.Workflow, rec.Status)
	closed := rec.Status == "closed"
	claimed := rec.AssignedTo != nil && *rec.AssignedTo != ""
	canClose := ticket.CanTransition(rec.Type, rec.Workflow, rec.Status, "closed") && rec.Resolution != nil
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: "ticket:claim:" + rec.ID, Label: "Claim", Style: discordgo.PrimaryButton, Disabled: !ok || closed || claimed},
			discordgo.Button{CustomID: "ticket:status:" + rec.ID, Label: "Set status", Style: discordgo.SecondaryButton, Disabled: !ok || closed},
			discordgo.Button{CustomID: "ticket:close:" + rec.ID, Label: "Close", Style: discordgo.DangerButton, Disabled: !canClose},
		}},
	}
}

func ticketSummary(rec ticket.Record) *discordgo.MessageEmbed {
	status, _ := ticket.StatusFor(rec.Type, rec.Workflow, rec.Status)
	label := rec.Status
	if status.Label != "" {
		label = status.Label
	}
	assigned := "Awaiting admin"
	if rec.AssignedTo != nil && *rec.AssignedTo != "" {
		assigned = "<@" + *rec.AssignedTo + ">"
	}
	next := ticket.NextStatuses(rec.Type, rec.Workflow, rec.Status)
	var nextLabels []string
	for _, s := range next {
		nextLabels = append(nextLabels, s.Label)
	}
	desc := "Only admins handle this ticket."
	if status.Note != "" {
		desc += "\n" + status.Note
	}
	return &discordgo.MessageEmbed{
		Title: rec.ID, Description: desc, Color: 0x4A2FBD, Footer: &discordgo.MessageEmbedFooter{Text: disclaimer},
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Status", Value: label, Inline: true},
			{Name: "Assigned admin", Value: assigned, Inline: true},
			{Name: "Region", Value: rec.Region, Inline: true},
			{Name: "Next", Value: strings.Join(nextLabels, " · ")},
		},
	}
}

func verifyPanel() *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{{
			Title: "✅ Verify to access the marketplace", Color: 0x2ECC71,
			Description: "Press **Verify** to acknowledge the rules and unlock marketplace channels.\nNew members start as User. Verify replaces User with Buyer.\nThis is not an identity check or a safety guarantee.",
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Admins will never ask you for", Value: "passwords · OTP · PIN · CVV · card numbers · API keys · private keys · seed phrases"},
			},
			Footer: &discordgo.MessageEmbedFooter{Text: disclaimer},
		}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: membership.VerifyButtonID, Label: "Verify", Style: discordgo.SuccessButton, Emoji: &discordgo.ComponentEmoji{Name: "✅"}},
			}},
		},
	}
}

func pickerRows(kind string, choices []membership.PickerChoice) []discordgo.MessageComponent {
	var rows []discordgo.MessageComponent
	for i := 0; i < len(choices); i += 5 {
		end := i + 5
		if end > len(choices) {
			end = len(choices)
		}
		var buttons []discordgo.MessageComponent
		for _, c := range choices[i:end] {
			buttons = append(buttons, discordgo.Button{
				CustomID: "rolepick:" + kind + ":" + c.Key, Label: c.Label, Style: discordgo.SecondaryButton,
				Emoji: &discordgo.ComponentEmoji{Name: c.Emoji},
			})
		}
		rows = append(rows, discordgo.ActionsRow{Components: buttons})
	}
	return rows
}

func languagePicker() *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Embeds:     []*discordgo.MessageEmbed{{Title: "Choose a language", Description: "Routing only. Not a safety guarantee.", Color: 0x3498DB}},
		Components: pickerRows("lang", membership.LanguageChoices),
	}
}

func regionPicker() *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		Embeds:     []*discordgo.MessageEmbed{{Title: "Choose a region", Description: "Routing only. Not a safety guarantee.", Color: 0x9B59B6}},
		Components: pickerRows("region", membership.RegionChoices),
	}
}

func slashCommands() []*discordgo.ApplicationCommand {
	admin := int64(discordgo.PermissionAdministrator)
	str := discordgo.ApplicationCommandOptionString
	user := discordgo.ApplicationCommandOptionUser
	integer := discordgo.ApplicationCommandOptionInteger
	out := []*discordgo.ApplicationCommand{}
	for _, c := range Commands {
		cmd := &discordgo.ApplicationCommand{Name: c.Name, Description: c.Description}
		if c.AdminOnly {
			cmd.DefaultMemberPermissions = &admin
		}
		switch c.Name {
		case "tickets", "mutasi":
			cmd.Options = []*discordgo.ApplicationCommandOption{{Type: integer, Name: "page", Description: "page"}}
		case "ticket-handoff":
			cmd.Options = []*discordgo.ApplicationCommandOption{
				{Type: user, Name: "admin", Description: "admin", Required: true},
				{Type: str, Name: "reason", Description: "reason", Required: true, MaxLength: ticket.TicketTextLimit},
			}
		case "ticket-resolution":
			cmd.Options = []*discordgo.ApplicationCommandOption{{Type: str, Name: "note", Description: "note", Required: true, MaxLength: ticket.TicketTextLimit}}
		case "lookup":
			cmd.Options = []*discordgo.ApplicationCommandOption{{Type: user, Name: "member", Description: "member", Required: true}}
		case "seller-approve":
			cmd.Options = []*discordgo.ApplicationCommandOption{{Type: str, Name: "reason", Description: "reason", Required: true, MaxLength: ticket.TicketTextLimit}}
		case "withdraw-paid":
			cmd.Options = []*discordgo.ApplicationCommandOption{{Type: str, Name: "reference", Description: "receipt reference", Required: true, MaxLength: ticket.PaymentRefLimit}}
		case "areapanel":
			cmd.Options = []*discordgo.ApplicationCommandOption{{
				Type: discordgo.ApplicationCommandOptionString, Name: "side",
				Description: "Which marketplace area this panel belongs to", Required: true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Seller Area", Value: membership.TierSeller},
					{Name: "Buyer Area", Value: membership.TierBuyer},
				},
			}}
		}
		out = append(out, cmd)
	}
	return out
}

func closeConfirm(rec ticket.Record) *discordgo.InteractionResponseData {
	ready := rec.Resolution != nil && ticket.CanTransition(rec.Type, rec.Workflow, rec.Status, "closed")
	note := "A saved resolution note is required. Use /ticket-resolution first."
	if rec.Resolution != nil {
		note = rec.Resolution.Text
	}
	return &discordgo.InteractionResponseData{
		Content: "Close ticket **" + rec.ID + "**? Confirm archives the thread. Cancel keeps it unchanged.",
		Embeds:  []*discordgo.MessageEmbed{{Title: "Resolution (member-visible)", Description: note, Color: 0x4A2FBD}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: "ticket:confirmclose:" + rec.ID, Label: "Confirm close", Style: discordgo.DangerButton, Disabled: !ready},
				discordgo.Button{CustomID: "ticket:cancelclose:" + rec.ID, Label: "Cancel", Style: discordgo.SecondaryButton},
			}},
		},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func paidConfirm(rec ticket.Record, reference string) *discordgo.InteractionResponseData {
	ready := ticket.IsWithdrawal(rec.Type, rec.Workflow) && rec.Status == "approved" && rec.Withdrawal != nil
	desc := "This request is not ready for payment confirmation."
	if ready {
		desc = "Ticket: " + rec.ID + "\nAmount: **" + ticket.FormatAmount(rec.Withdrawal.AmountMinor, rec.Withdrawal.Currency) + " " + rec.Withdrawal.Currency + "**\nSale: " + rec.Withdrawal.SaleReference + "\nReference: " + reference
	}
	return &discordgo.InteractionResponseData{
		Content: "Confirm only if the external payment already succeeded. This records an outgoing transaction; it does not transfer money. Expires in 120 seconds.",
		Embeds:  []*discordgo.MessageEmbed{{Title: "Confirm external withdrawal payment", Description: desc, Color: 0xE67E22}},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.Button{CustomID: "ticket:confirmpaid:" + rec.ID, Label: "Confirm payment recorded", Style: discordgo.DangerButton, Disabled: !ready},
				discordgo.Button{CustomID: "ticket:cancelpaid:" + rec.ID, Label: "Cancel", Style: discordgo.SecondaryButton},
			}},
		},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func formatQueue(recs []ticket.Record) string {
	if len(recs) == 0 {
		return "Queue is empty."
	}
	var b strings.Builder
	b.WriteString("Active tickets:\n")
	for _, rec := range recs {
		fmt.Fprintf(&b, "• `%s` %s · %s · <#%s>\n", rec.ID, rec.Status, rec.Type, rec.ThreadID)
	}
	return b.String()
}

// memberPanel explains the two marketplace roles and shows how many members
// currently hold each one. The counts come from Discord's own roles, not from
// the stored tier column, because the roles are what actually decide who can
// see which channels.
func memberPanel(counts map[string]int, total int) *discordgo.InteractionResponseData {
	side := func(tier string) string {
		return itoa(counts[tier]) + " member(s)"
	}
	return &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{{
			Title: "🧭 Buyer and Seller are separate areas", Color: 0x3498DB,
			Description: "This server has two marketplace roles. They are mutually exclusive: " +
				"granting one removes the other, so nobody is both.\n" +
				"Each area is hidden from the other side — a Buyer cannot read the Seller " +
				"channels, and a Seller cannot read the Buyer channels.",
			Fields: []*discordgo.MessageEmbedField{
				{Name: "🛍️ Buyer", Value: side(membership.TierBuyer) + "\nMarketplace access. Buys. Cannot see the Seller Area."},
				{Name: "🏪 Seller", Value: side(membership.TierSeller) + "\nSells and can withdraw. Cannot see the Buyer Area."},
				{Name: "How to get one", Value: "Press **Verify** in the #verification channel to become a Buyer.\n" +
					"Seller is granted by an admin after a seller application."},
				{Name: "Everyone else", Value: itoa(total) + " member(s) in the guild hold neither role yet."},
			},
			Footer: &discordgo.MessageEmbedFooter{Text: disclaimer},
		}},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

// areaPanel is the standing message inside a marketplace area. It tells the
// member why they can see this area and what they cannot see, and gives them a
// button to act without leaving the channel. The buttons open a ticket modal
// through the same `ticket:request:` path the ticket panel uses; eligibility is
// enforced when the ticket is created, so a button never has to be hidden.
func areaPanel(side string) *discordgo.InteractionResponseData {
	var title, body string
	var buttons []discordgo.MessageComponent
	switch side {
	case membership.TierSeller:
		title = "🏪 Seller Area"
		body = "You can see this area because you hold the **Seller** role.\n" +
			"The **Buyer Area** is hidden from you, and this area is hidden from Buyers — " +
			"the two sides are kept apart on purpose.\n\n" +
			"**Withdrawals are manual.** Approval is not payment: an admin verifies the " +
			"external sale, then records one outgoing entry. There is no automatic transfer."
		buttons = []discordgo.MessageComponent{
			discordgo.Button{CustomID: "ticket:request:withdraw", Label: "Request a withdrawal", Style: discordgo.PrimaryButton, Emoji: &discordgo.ComponentEmoji{Name: "📤"}},
			discordgo.Button{CustomID: "ticket:request:sale", Label: "Coordinate a sale", Style: discordgo.SecondaryButton, Emoji: &discordgo.ComponentEmoji{Name: "🏷️"}},
		}
	default:
		title = "🛍️ Buyer Area"
		body = "You can see this area because you hold the **Buyer** role.\n" +
			"The **Seller Area** is hidden from you, and this area is hidden from Sellers — " +
			"the two sides are kept apart on purpose.\n\n" +
			"**Seller access is not self-service.** An admin grants it after a seller " +
			"application, and it replaces your Buyer role."
		buttons = []discordgo.MessageComponent{
			discordgo.Button{CustomID: "ticket:request:purchase", Label: "Open a purchase ticket", Style: discordgo.PrimaryButton, Emoji: &discordgo.ComponentEmoji{Name: "🛒"}},
			discordgo.Button{CustomID: "ticket:request:buyer-support", Label: "Get help", Style: discordgo.SecondaryButton, Emoji: &discordgo.ComponentEmoji{Name: "💬"}},
		}
	}
	return &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{{
			Title: title, Color: 0x3498DB, Description: body,
			Fields: []*discordgo.MessageEmbedField{
				{Name: "Never share", Value: "passwords · OTP · PIN · CVV · card numbers · API keys · private keys · seed phrases"},
			},
			Footer: &discordgo.MessageEmbedFooter{Text: disclaimer},
		}},
		Components:      []discordgo.MessageComponent{discordgo.ActionsRow{Components: buttons}},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}
