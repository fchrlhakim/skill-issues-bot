package discordbot

import (
	"strings"
	"testing"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/ticket"

	"github.com/bwmarrin/discordgo"
)

// buttonIDs pulls the custom_ids out of a panel response.
func buttonIDs(p *discordgo.InteractionResponseData) []string {
	var out []string
	for _, row := range p.Components {
		ar, ok := row.(discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, c := range ar.Components {
			switch b := c.(type) {
			case discordgo.Button:
				out = append(out, b.CustomID)
			case *discordgo.Button:
				out = append(out, b.CustomID)
			}
		}
	}
	return out
}

// A button that names a ticket type the catalog does not know opens a modal
// for a type that cannot be created, so the member gets an error after filling
// the form in. Every request button must resolve.
func TestAreaPanelButtonsNameRealTicketTypes(t *testing.T) {
	for _, side := range []string{membership.TierBuyer, membership.TierSeller} {
		ids := buttonIDs(areaPanel(side))
		if len(ids) == 0 {
			t.Fatalf("%s panel has no buttons", side)
		}
		for _, id := range ids {
			if !strings.HasPrefix(id, "ticket:request:") {
				t.Errorf("%s: %q is not a ticket request button", side, id)
				continue
			}
			key := strings.TrimPrefix(id, "ticket:request:")
			if _, ok := ticket.TypeByKey(key); !ok {
				t.Errorf("%s: button %q names ticket type %q which does not exist", side, id, key)
			}
		}
	}
}

// The withdraw button is only meaningful to a Seller, and the purchase button
// only to a Buyer. Putting one on the wrong panel gives a member a button that
// is refused after they fill in the modal.
func TestAreaPanelButtonsMatchTheSide(t *testing.T) {
	seller := strings.Join(buttonIDs(areaPanel(membership.TierSeller)), " ")
	if !strings.Contains(seller, "ticket:request:withdraw") {
		t.Error("seller panel should offer a withdrawal request")
	}
	if strings.Contains(seller, "ticket:request:purchase") {
		t.Error("seller panel must not offer the buyer purchase ticket")
	}
	buyer := strings.Join(buttonIDs(areaPanel(membership.TierBuyer)), " ")
	if !strings.Contains(buyer, "ticket:request:purchase") {
		t.Error("buyer panel should offer a purchase ticket")
	}
	if strings.Contains(buyer, "ticket:request:withdraw") {
		t.Error("buyer panel must not offer the seller-only withdrawal ticket")
	}
}

// The withdraw ticket is gated to Seller, so the buyer panel must never expose
// it. This pins the panel to the same rule the service enforces on create.
func TestBuyerPanelAvoidsSellerOnlyTickets(t *testing.T) {
	for _, id := range buttonIDs(areaPanel(membership.TierBuyer)) {
		key := strings.TrimPrefix(id, "ticket:request:")
		if ticket.GatedTypes[key] && !membership.EligibleTicketApplicant(key, []string{membership.RoleBuyer}) {
			t.Errorf("buyer panel exposes %q, which a Buyer cannot open", key)
		}
	}
}

// Each area panel must state that the other side is hidden, so the member
// understands why they cannot see the other area.
func TestAreaPanelStatesTheOtherSideIsHidden(t *testing.T) {
	for _, tc := range []struct{ side, other string }{
		{membership.TierBuyer, "Seller Area"},
		{membership.TierSeller, "Buyer Area"},
	} {
		p := areaPanel(tc.side)
		if len(p.Embeds) != 1 {
			t.Fatalf("%s: embeds=%d", tc.side, len(p.Embeds))
		}
		if !strings.Contains(p.Embeds[0].Description, tc.other) {
			t.Errorf("%s panel does not mention the hidden %s", tc.side, tc.other)
		}
		if !strings.Contains(p.Embeds[0].Description, "hidden") {
			t.Errorf("%s panel does not say the other side is hidden", tc.side)
		}
	}
}

// The verify reply has to name the side the member is now on. "Membership
// confirmed" left a new member unable to tell whether they were a Buyer or a
// Seller, and the two areas look identical until one opens.
func TestTierLabelNamesTheSide(t *testing.T) {
	for _, tc := range []struct {
		roles []string
		want  string
	}{
		{[]string{membership.RoleBuyer}, "Buyer"},
		{[]string{membership.RoleSeller}, "Seller"},
		{[]string{membership.RoleUser}, "member without marketplace access"},
		{[]string{membership.RoleBuyer, membership.RoleSeller}, "member without marketplace access"},
		{nil, "member without marketplace access"},
	} {
		if got := tierLabel(tc.roles); got != tc.want {
			t.Errorf("tierLabel(%v) = %q, want %q", tc.roles, got, tc.want)
		}
	}
}

// Someone who presses Verify a second time is usually looking for a channel
// they cannot see. The reply must name the side, name the hidden area, and give
// the route to the other side, instead of a bare "nothing changed".
func TestAlreadyVerifiedReplyIsActionable(t *testing.T) {
	buyer := alreadyVerifiedReply("Buyer")
	for _, want := range []string{"already a **Buyer**", "Buyer Area is open", "Seller Area stays hidden", "#open-a-ticket"} {
		if !strings.Contains(buyer, want) {
			t.Errorf("buyer reply missing %q:\n%s", want, buyer)
		}
	}
	seller := alreadyVerifiedReply("Seller")
	for _, want := range []string{"already a **Seller**", "Seller Area is open", "Buyer Area stays hidden"} {
		if !strings.Contains(seller, want) {
			t.Errorf("seller reply missing %q:\n%s", want, seller)
		}
	}
	// A member with no tier must not be told they have an open area.
	none := alreadyVerifiedReply(tierLabel([]string{membership.RoleUser}))
	if strings.Contains(none, "Area is open to you") {
		t.Errorf("unverified member must not be promised an area:\n%s", none)
	}
}
