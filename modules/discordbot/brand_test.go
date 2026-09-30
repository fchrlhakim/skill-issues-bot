package discordbot

import (
	"strings"
	"testing"
)

// The ticket panel is a standing Discord message. Shipping the old Node
// project's Skillissue.ai name made the live panel look like a different
// product from skill-issues.dev. Member-facing copy must use Skill Issues.
func TestTicketPanelUsesSkillIssuesBrand(t *testing.T) {
	p := ticketPanel()
	if len(p.Embeds) != 1 {
		t.Fatalf("embeds=%d", len(p.Embeds))
	}
	e := p.Embeds[0]
	if e.Title != "Open a Ticket — Skill Issues" {
		t.Fatalf("title=%q", e.Title)
	}
	if e.Footer == nil || e.Footer.Text != disclaimer {
		t.Fatalf("footer=%v", e.Footer)
	}
	for _, s := range []string{e.Title, e.Description, e.Footer.Text, disclaimer, SafetyCopy} {
		if strings.Contains(s, "Skillissue.ai") {
			t.Fatalf("member-facing copy still names Skillissue.ai: %q", s)
		}
		if !strings.Contains(s, "Skill Issues") && s == e.Title {
			t.Fatalf("title does not name Skill Issues: %q", s)
		}
	}
	if !strings.Contains(disclaimer, "Skill Issues") {
		t.Fatalf("disclaimer=%q", disclaimer)
	}
	if !strings.Contains(SafetyCopy, "Skill Issues") {
		t.Fatalf("SafetyCopy=%q", SafetyCopy)
	}
}
