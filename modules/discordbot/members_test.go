package discordbot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	for _, want := range []string{"Buyers — 4", "Sellers — 2", "No role yet — 9"} {
		if !strings.Contains(body, want) {
			t.Fatalf("panel missing %q in:\n%s", want, body)
		}
	}
}

// A zero count must still render. Omitting the line would leave a reader unable
// to tell "no sellers" from "the count was not reported".
func TestMemberPanelRendersZeroCounts(t *testing.T) {
	p := memberPanel(map[string]int{membership.TierBuyer: 0, membership.TierSeller: 0}, 0)
	body := ""
	for _, f := range p.Embeds[0].Fields {
		body += f.Name + "\n"
	}
	for _, want := range []string{"Buyers — 0", "Sellers — 0", "No role yet — 0"} {
		if !strings.Contains(body, want) {
			t.Fatalf("panel missing %q in:\n%s", want, body)
		}
	}
}

// The panel is edited in place, so its location has to survive a restart. If
// this round trip fails the bot posts a NEW panel on every refresh and the
// channel fills with stale counts.
func TestPanelStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MEMBER_PANEL_FILE", filepath.Join(dir, "member-panel.json"))

	if _, ok := loadPanelState(); ok {
		t.Fatal("empty file should report no panel")
	}
	want := panelState{ChannelID: "c1", MessageID: "m1", UpdatedAt: time.Now().UTC().Truncate(time.Second)}
	if err := savePanelState(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok := loadPanelState()
	if !ok {
		t.Fatal("saved panel not found")
	}
	if got.ChannelID != want.ChannelID || got.MessageID != want.MessageID {
		t.Fatalf("got %+v want %+v", got, want)
	}
	// A half-written file must not be treated as a valid panel, or the bot would
	// try to edit a message id it never posted.
	if err := os.WriteFile(os.Getenv("MEMBER_PANEL_FILE"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadPanelState(); ok {
		t.Fatal("corrupt state must not report a panel")
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
