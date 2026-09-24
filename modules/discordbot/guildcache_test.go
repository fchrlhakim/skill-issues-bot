package discordbot

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
)

// guildCacheHarness points discordgo at a local server and counts how many times
// the guild endpoint is actually hit, so a cache claim can be measured instead of
// asserted.
func guildCacheHarness(t *testing.T) (*discordgo.Session, *int64) {
	t.Helper()
	var calls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"g1","name":"guild","owner_id":"owner-1"}`)
	}))
	t.Cleanup(server.Close)

	original := discordgo.EndpointGuild
	discordgo.EndpointGuild = func(guildID string) string { return server.URL + "/guilds/" + guildID }
	t.Cleanup(func() { discordgo.EndpointGuild = original })

	session, err := discordgo.New("Bot token")
	if err != nil {
		t.Fatal(err)
	}
	return session, &calls
}

// The guild object carries the owner id, which every interaction needs for its
// actor. Fetching it per interaction is a REST round trip to Discord on every
// button click and slash command, so it must be served from cache.
func TestGuildIsFetchedOncePerTTL(t *testing.T) {
	session, calls := guildCacheHarness(t)
	bot := &Bot{guildID: "g1", log: testLogger()}

	for i := range 5 {
		if guild := bot.guild(session); guild == nil || guild.OwnerID != "owner-1" {
			t.Fatalf("call %d returned %#v", i, guild)
		}
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("guild endpoint hit %d times for 5 lookups; want 1", got)
	}
}

// isAdmin is consulted for the acting member on every interaction, so it must not
// add its own guild fetch on top of the cached one.
func TestIsAdminUsesCachedGuild(t *testing.T) {
	session, calls := guildCacheHarness(t)
	bot := &Bot{guildID: "g1", log: testLogger()}

	member := &discordgo.Member{User: &discordgo.User{ID: "owner-1"}}
	if !bot.isAdmin(session, member) {
		t.Fatal("the guild owner must be an admin")
	}
	for range 3 {
		bot.isAdmin(session, member)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("guild endpoint hit %d times across 4 isAdmin calls; want 1", got)
	}
}

// A failed fetch must not be cached: pinning an empty guild for the whole TTL
// would silently deny every admin for two minutes.
func TestGuildCacheDoesNotCacheFailure(t *testing.T) {
	var calls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"g1","name":"guild","owner_id":"owner-1"}`)
	}))
	defer server.Close()

	original := discordgo.EndpointGuild
	discordgo.EndpointGuild = func(guildID string) string { return server.URL + "/guilds/" + guildID }
	defer func() { discordgo.EndpointGuild = original }()

	session, err := discordgo.New("Bot token")
	if err != nil {
		t.Fatal(err)
	}
	bot := &Bot{guildID: "g1", log: testLogger()}

	if guild := bot.guild(session); guild != nil {
		t.Fatalf("failed fetch returned %#v", guild)
	}
	guild := bot.guild(session)
	if guild == nil || guild.OwnerID != "owner-1" {
		t.Fatalf("retry after a failure returned %#v; a failed fetch was cached", guild)
	}
}

// TestAdminIDsFiltersBotsAndCaches covers the roster the create/handoff paths
// hand a ticket to. Two properties matter: bots are never treated as admins, and
// the walk (one permission resolution per member) is not repeated per
// interaction. The role id used below resolves through the State cache, so no
// role endpoint is involved.
func TestAdminIDsFiltersBotsAndCaches(t *testing.T) {
	var guildCalls, memberCalls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/guilds/g1":
			atomic.AddInt64(&guildCalls, 1)
			fmt.Fprint(w, `{"id":"g1","name":"guild","owner_id":"owner-1","roles":[{"id":"r-admin","name":"Staff","permissions":"8"}]}`)
		case "/guilds/g1/members":
			atomic.AddInt64(&memberCalls, 1)
			fmt.Fprint(w, `[
				{"user":{"id":"owner-1","username":"owner"},"roles":[]},
				{"user":{"id":"admin-1","username":"admin"},"roles":["r-admin"]},
				{"user":{"id":"bot-1","username":"bot","bot":true},"roles":["r-admin"]},
				{"user":{"id":"member-1","username":"member"},"roles":[]}
			]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	originalGuild, originalMembers := discordgo.EndpointGuild, discordgo.EndpointGuildMembers
	discordgo.EndpointGuild = func(guildID string) string { return server.URL + "/guilds/" + guildID }
	discordgo.EndpointGuildMembers = func(guildID string) string { return server.URL + "/guilds/" + guildID + "/members" }
	defer func() {
		discordgo.EndpointGuild = originalGuild
		discordgo.EndpointGuildMembers = originalMembers
	}()

	session, err := discordgo.New("Bot token")
	if err != nil {
		t.Fatal(err)
	}
	// The role cache must be primed; the handler under test resolves roles
	// through State, never through a per-member REST call.
	session.State.GuildAdd(&discordgo.Guild{ID: "g1", Roles: []*discordgo.Role{{ID: "r-admin", Name: "Staff", Permissions: discordgo.PermissionAdministrator}}})

	bot := &Bot{guildID: "g1", log: testLogger()}
	ids := bot.adminIDs(session)
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if !got["owner-1"] || !got["admin-1"] {
		t.Fatalf("roster missing real admins: %v", ids)
	}
	if got["bot-1"] {
		t.Fatalf("a bot was treated as an admin: %v", ids)
	}
	if got["member-1"] {
		t.Fatalf("a plain member was treated as an admin: %v", ids)
	}

	for range 4 {
		bot.adminIDs(session)
	}
	if memberCalls != 1 {
		t.Fatalf("member list fetched %d times across 5 calls; want 1", memberCalls)
	}
	if guildCalls != 1 {
		t.Fatalf("guild fetched %d times; want 1", guildCalls)
	}
}
