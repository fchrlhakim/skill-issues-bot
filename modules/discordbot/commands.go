package discordbot

// Slash commands mirrored from skillissue-discord/src/lib/bot-commands.mjs.
// Discord default visibility is not authorization; handlers recheck Administrator.

type Command struct {
	Name        string
	Description string
	AdminOnly   bool
}

var Commands = []Command{
	{Name: "panel", Description: "Post the ticket panel in this channel (admin only)", AdminOnly: true},
	{Name: "rolepanel", Description: "Post the language and region pickers here (admin only)", AdminOnly: true},
	{Name: "verifypanel", Description: "Post the marketplace access panel here (admin only)", AdminOnly: true},
	{Name: "testwelcome", Description: "Preview your own welcome card and message (admin only)", AdminOnly: true},
	{Name: "tickets", Description: "View the active ticket queue (admin only)", AdminOnly: true},
	{Name: "ticket-status", Description: "Show your ticket status, assignment, and next steps"},
	{Name: "ticket-handoff", Description: "Assign this ticket to another human admin (admin only)", AdminOnly: true},
	{Name: "ticket-resolution", Description: "Save the member-visible resolution note before closing (admin only)", AdminOnly: true},
	{Name: "whoami", Description: "Show your roles and access"},
	{Name: "safety", Description: "Read transaction safety rules and prohibited credentials"},
	{Name: "lookup", Description: "Look up a member's standing and account age (admin only)", AdminOnly: true},
	{Name: "withdraw", Description: "Request a manual seller withdrawal review; no automatic transfer"},
	{Name: "mutasi", Description: "View only your own admin-confirmed outgoing records"},
	{Name: "seller-approve", Description: "Approve this seller application and grant Seller (admin only)", AdminOnly: true},
	{Name: "withdraw-paid", Description: "Record an already completed external withdrawal payment (admin only)", AdminOnly: true},
}

const SafetyCopy = "Admins never ask for passwords, OTP, PIN, CVV, full card numbers, API keys, private keys, or seed phrases. " +
	"Skillissue.ai is an independent intermediary: it does not hold funds, process payments, or guarantee transactions. " +
	"Every deal goes through a ticket — never DMs."

const VerifyButtonID = "verify:accept"
