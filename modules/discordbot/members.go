package discordbot

import (
	"context"
	"fmt"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/primitive"

	"github.com/bwmarrin/discordgo"
)

// reconcileTiers makes the stored tier agree with the roles Discord actually
// holds. The roles are the source of truth: they decide which channels a
// member can see, so a stored tier that disagrees is a report of something
// that is not true.
//
// It runs for two reasons:
//
//  1. Granting a role and recording the tier were separate steps, so a
//     membership change could land in Discord without reaching the database
//     (seller-approve granted Seller and never stored it).
//  2. guild_members is only written when someone joins or verifies, so members
//     who predate the bot have no row at all.
//
// It never changes a role and never deletes a row for someone still in the
// guild. The only deletion is a stored row whose member has left.
func (b *Bot) reconcileTiers(ctx context.Context) (int, int, error) {
	stored, err := b.members.List(ctx)
	if err != nil {
		return 0, 0, err
	}
	roles, err := b.session.GuildRoles(b.guildID)
	if err != nil {
		return 0, 0, err
	}
	roleName := make(map[string]string, len(roles))
	for _, r := range roles {
		roleName[r.ID] = r.Name
	}
	members, err := b.guildMembers()
	if err != nil {
		return 0, 0, err
	}

	byID := make(map[string]primitive.GuildMember, len(stored))
	for _, m := range stored {
		byID[m.DiscordID] = m
	}

	repaired := 0
	for _, m := range members {
		if m.User == nil || m.User.Bot {
			continue
		}
		names := make([]string, 0, len(m.Roles))
		for _, id := range m.Roles {
			if n, ok := roleName[id]; ok {
				names = append(names, n)
			}
		}
		tier := membership.MemberTier(names)
		// A conflicting pair cannot be represented as a tier, and it needs a
		// human to resolve, so it is reported by /lookup rather than guessed.
		if tier == membership.TierConflict {
			continue
		}
		row, known := byID[m.User.ID]
		restricted := membership.Restricted(names)
		// A member with no tier role is "none". Recording that only matters if
		// a row already exists: leaving the old tier there would report someone
		// as a Buyer after their role was taken away. Members who never had a
		// row are skipped, so the table does not grow to hold every lurker.
		if tier == membership.TierNone {
			if !known || (row.Tier == membership.TierNone && row.Restricted == restricted) {
				continue
			}
			if _, err := b.members.Upsert(ctx, m.User.ID, row.Username, membership.TierNone, row.AccountAgeDays, restricted); err != nil {
				return repaired, 0, err
			}
			repaired++
			continue
		}
		age := row.AccountAgeDays
		if !known {
			age = accountAgeDays(m.User)
		}
		if known && row.Tier == tier && row.Restricted == restricted {
			continue
		}
		username := row.Username
		if username == "" {
			username = m.User.Username
		}
		if _, err := b.members.Upsert(ctx, m.User.ID, username, tier, age, restricted); err != nil {
			return repaired, 0, err
		}
		repaired++
	}

	// A stored row for someone no longer in the guild inflates every count.
	pruned := 0
	present := make(map[string]bool, len(members))
	for _, m := range members {
		if m.User != nil {
			present[m.User.ID] = true
		}
	}
	for _, row := range stored {
		if present[row.DiscordID] {
			continue
		}
		if err := b.members.Delete(ctx, row.DiscordID); err != nil {
			return repaired, pruned, err
		}
		pruned++
	}
	return repaired, pruned, nil
}

// guildMembers pages the full member list. The gateway only caches members it
// has seen, so a reconciliation must ask Discord directly or it would compare
// against a partial list.
func (b *Bot) guildMembers() ([]*discordgo.Member, error) {
	var out []*discordgo.Member
	after := ""
	for {
		page, err := b.session.GuildMembers(b.guildID, after, 1000)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if len(page) < 1000 {
			return out, nil
		}
		after = page[len(page)-1].User.ID
	}
}

// liveTierCounts counts Buyer and Seller from Discord's own roles. The stored
// tier column is not used: it is derived, and a stale row must never be
// reported as a headcount.
func (b *Bot) liveTierCounts() (map[string]int, int, error) {
	members, err := b.guildMembers()
	if err != nil {
		return nil, 0, err
	}
	roles, err := b.session.GuildRoles(b.guildID)
	if err != nil {
		return nil, 0, err
	}
	roleName := make(map[string]string, len(roles))
	for _, r := range roles {
		roleName[r.ID] = r.Name
	}
	counts, neither := tierCounts(members, roleName)
	return counts, neither, nil
}

// tierCounts counts members per marketplace role. Bots are excluded: they hold
// no tier, and counting them would inflate the "everyone else" figure. A member
// holding both roles is counted in neither, because MemberTier reports it as a
// conflict that needs an admin, not as a Buyer or a Seller.
func tierCounts(members []*discordgo.Member, roleName map[string]string) (map[string]int, int) {
	counts := map[string]int{membership.TierBuyer: 0, membership.TierSeller: 0}
	neither := 0
	for _, m := range members {
		if m.User == nil || m.User.Bot {
			continue
		}
		names := make([]string, 0, len(m.Roles))
		for _, id := range m.Roles {
			if n, ok := roleName[id]; ok {
				names = append(names, n)
			}
		}
		switch membership.MemberTier(names) {
		case membership.TierBuyer:
			counts[membership.TierBuyer]++
		case membership.TierSeller:
			counts[membership.TierSeller]++
		default:
			neither++
		}
	}
	return counts, neither
}

// setTier changes a member's marketplace role and records the resulting tier in
// the same call. exclusiveTier alone only touches Discord, which left the
// stored tier stale — the seller-approve path granted Seller and never wrote it.
func (b *Bot) setTier(s *discordgo.Session, userID, target, username string) error {
	if err := b.exclusiveTier(s, userID, target); err != nil {
		return err
	}
	member, err := s.GuildMember(b.guildID, userID)
	if err != nil {
		return err
	}
	age := accountAgeDays(member.User)
	if member.User == nil {
		return fmt.Errorf("member %s has no user", userID)
	}
	if username == "" {
		username = member.User.Username
	}
	_, err = b.members.Upsert(context.Background(), userID, username, target, age, membership.Restricted(b.roleNames(s, member)))
	return err
}

// tierLabel names the marketplace side a member is on, for messages that have to
// tell someone what they are. MemberTier reports "none" for an unverified member
// and "conflict" for someone holding two tier roles; neither is a side, and
// calling them a Buyer would be wrong.
func tierLabel(roles []string) string {
	switch membership.MemberTier(roles) {
	case membership.TierBuyer:
		return "Buyer"
	case membership.TierSeller:
		return "Seller"
	default:
		return "member without marketplace access"
	}
}

func (b *Bot) reconcileSummary(repaired, pruned int) string {
	return fmt.Sprintf("Tier sync complete. Repaired %d stored tier(s), pruned %d row(s) for members who left.", repaired, pruned)
}
