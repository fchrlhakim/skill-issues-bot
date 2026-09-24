package discordbot

import (
	"testing"

	"go-starter-kit/modules/membership"

	"github.com/bwmarrin/discordgo"
)

func member(id string, bot bool, roleIDs ...string) *discordgo.Member {
	return &discordgo.Member{User: &discordgo.User{ID: id, Username: "u" + id, Bot: bot}, Roles: roleIDs}
}

func TestTierCountsSeparatesBuyerAndSeller(t *testing.T) {
	roles := map[string]string{
		"r-buyer": "Buyer", "r-seller": "Seller", "r-user": "User",
		"r-everyone": "@everyone", "r-other": "Gamer",
	}
	members := []*discordgo.Member{
		member("1", false, "r-buyer", "r-everyone"),
		member("2", false, "r-buyer", "r-other"),
		member("3", false, "r-seller"),
		member("4", false, "r-user"),              // verified but no tier role yet
		member("5", false),                        // plain member
		member("6", true, "r-buyer"),              // a bot must not be counted
		member("7", false, "r-buyer", "r-seller"), // conflict: both roles
	}
	counts, neither := tierCounts(members, roles)
	if counts[membership.TierBuyer] != 2 {
		t.Fatalf("buyer=%d want 2", counts[membership.TierBuyer])
	}
	if counts[membership.TierSeller] != 1 {
		t.Fatalf("seller=%d want 1", counts[membership.TierSeller])
	}
	// user, plain, and the conflicting pair — the bot is excluded.
	if neither != 3 {
		t.Fatalf("neither=%d want 3", neither)
	}
}

// A conflicting member must never be reported as a Buyer or a Seller: the roles
// are what decide channel access, and a conflict means the member's access is
// undecided, not that they belong to one side.
func TestTierCountsConflictIsNotATier(t *testing.T) {
	roles := map[string]string{"a": "Buyer", "b": "Seller"}
	counts, neither := tierCounts([]*discordgo.Member{member("1", false, "a", "b")}, roles)
	if counts[membership.TierBuyer] != 0 || counts[membership.TierSeller] != 0 {
		t.Fatalf("conflict counted as a tier: %+v", counts)
	}
	if neither != 1 {
		t.Fatalf("neither=%d want 1", neither)
	}
}

// The panel's counts must equal the roles actually held, so an empty guild
// reports zeros rather than omitting the lines.
func TestMemberPanelRendersBothTiersAndCounts(t *testing.T) {
	p := memberPanel(map[string]int{membership.TierBuyer: 4, membership.TierSeller: 2}, 9)
	if len(p.Embeds) != 1 {
		t.Fatalf("embeds=%d", len(p.Embeds))
	}
	body := ""
	for _, f := range p.Embeds[0].Fields {
		body += f.Name + "|" + f.Value + "\n"
	}
	for _, want := range []string{"4 member(s)", "2 member(s)", "9 member(s)"} {
		if !contains(body, want) {
			t.Fatalf("panel missing %q in:\n%s", want, body)
		}
	}
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
