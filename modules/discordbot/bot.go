package discordbot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/ticket"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type Bot struct {
	session    *discordgo.Session
	guildID    string
	tickets    ticket.ServiceInterface
	members    membership.ServiceInterface
	log        *logrus.Logger
	pending    sync.Map
	channelIDs map[string]string
}

func New(token, guildID string, tickets ticket.ServiceInterface, members membership.ServiceInterface, log *logrus.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
	bot := &Bot{session: session, guildID: guildID, tickets: tickets, members: members, log: log, channelIDs: map[string]string{}}
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onInteraction)
	session.AddHandler(bot.onJoin)
	session.AddHandler(bot.onLeave)
	return bot, nil
}

func (b *Bot) Open() error {
	if err := b.session.Open(); err != nil {
		return err
	}
	_, err := b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, b.guildID, slashCommands())
	return err
}

func (b *Bot) Close() error {
	return b.session.Close()
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	b.log.Infof("discord bot online as %s in guild %s", r.User.Username, b.guildID)
	b.refreshChannels()
}

func (b *Bot) refreshChannels() {
	guild, err := b.session.Guild(b.guildID)
	if err != nil {
		return
	}
	channels, err := b.session.GuildChannels(guild.ID)
	if err != nil {
		return
	}
	keys := map[string]string{
		"welcome": "welcome", "rules": "rules", "how-it-works": "how-it-works",
		"verification": "verification", "open-a-ticket": "open-a-ticket",
		"roles-and-languages": "roles-and-languages", "staff-log": "staff-log",
		"audit-log": "audit-log", "goodbye": "goodbye",
	}
	next := map[string]string{}
	for _, ch := range channels {
		for key := range keys {
			if ch.Name == key || strings.Contains(ch.Name, key) {
				next[key] = ch.ID
			}
		}
	}
	b.channelIDs = next
}

func (b *Bot) channel(key string) string {
	return b.channelIDs[key]
}

func (b *Bot) ephemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: discordgo.MessageFlagsEphemeral, AllowedMentions: &discordgo.MessageAllowedMentions{}},
	})
}

func (b *Bot) followup(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	_, _ = s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content, Flags: discordgo.MessageFlagsEphemeral, AllowedMentions: &discordgo.MessageAllowedMentions{}})
}

func (b *Bot) deferEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral},
	})
}

func (b *Bot) edit(s *discordgo.Session, i *discordgo.InteractionCreate, content string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	data := &discordgo.WebhookEdit{Content: &content, Embeds: &embeds, Components: &components}
	_, _ = s.InteractionResponseEdit(i.Interaction, data)
}

func (b *Bot) audit(guildID, event, text string) {
	ch := b.channel("audit-log")
	if ch == "" {
		ch = b.channel("staff-log")
	}
	if ch == "" {
		return
	}
	_, _ = b.session.ChannelMessageSendEmbed(ch, &discordgo.MessageEmbed{Title: "audit · " + event, Description: text, Color: 0x34495E})
}

func (b *Bot) staff(text string) {
	ch := b.channel("staff-log")
	if ch == "" {
		return
	}
	_, _ = b.session.ChannelMessageSend(ch, "`"+time.Now().UTC().Format(time.RFC3339)+"` "+text)
}

func (b *Bot) roleNames(s *discordgo.Session, m *discordgo.Member) []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.Roles))
	for _, id := range m.Roles {
		if role, err := s.State.Role(b.guildID, id); err == nil {
			out = append(out, role.Name)
		}
	}
	return out
}

func (b *Bot) isAdmin(s *discordgo.Session, m *discordgo.Member) bool {
	if m == nil {
		return false
	}
	guild, err := s.Guild(b.guildID)
	if err != nil {
		return false
	}
	if m.User != nil && m.User.ID == guild.OwnerID {
		return true
	}
	perms, err := s.UserChannelPermissions(m.User.ID, b.channel("open-a-ticket"))
	if err != nil {
		perms = 0
		for _, id := range m.Roles {
			if role, e := s.State.Role(b.guildID, id); e == nil && role.Permissions&discordgo.PermissionAdministrator != 0 {
				return true
			}
		}
	}
	return perms&discordgo.PermissionAdministrator != 0
}

func (b *Bot) actor(s *discordgo.Session, m *discordgo.Member) ticket.Actor {
	username, tag := "", ""
	id := ""
	if m != nil && m.User != nil {
		id, username, tag = m.User.ID, m.User.Username, m.User.String()
	}
	guild, _ := s.Guild(b.guildID)
	owner := ""
	if guild != nil {
		owner = guild.OwnerID
	}
	return ticket.Actor{
		ID: id, Username: username, Tag: tag, Roles: b.roleNames(s, m),
		IsAdmin: b.isAdmin(s, m), OwnerID: owner, AvailableAdmins: b.adminIDs(s),
	}
}

func (b *Bot) adminIDs(s *discordgo.Session) []string {
	guild, err := s.Guild(b.guildID)
	if err != nil {
		return nil
	}
	ids := []string{guild.OwnerID}
	members, err := s.GuildMembers(b.guildID, "", 1000)
	if err != nil {
		return ids
	}
	seen := map[string]bool{guild.OwnerID: true}
	for _, m := range members {
		if m.User == nil || m.User.Bot {
			continue
		}
		if b.isAdmin(s, m) && !seen[m.User.ID] {
			seen[m.User.ID] = true
			ids = append(ids, m.User.ID)
		}
	}
	return ids
}

func (b *Bot) lock(key string) bool {
	_, loaded := b.pending.LoadOrStore(key, true)
	return !loaded
}

func (b *Bot) unlock(key string) {
	if key != "" {
		b.pending.Delete(key)
	}
}

func (b *Bot) findRole(s *discordgo.Session, name string) *discordgo.Role {
	roles, err := s.GuildRoles(b.guildID)
	if err != nil {
		return nil
	}
	var found *discordgo.Role
	for _, role := range roles {
		if role.Name == name {
			if found != nil {
				return nil
			}
			found = role
		}
	}
	return found
}

func (b *Bot) exclusiveTier(s *discordgo.Session, userID, target string) error {
	member, err := s.GuildMember(b.guildID, userID)
	if err != nil {
		return err
	}
	current := b.roleNames(s, member)
	add, remove, ok := membership.ExclusiveGrant(current, target)
	if !ok {
		return fmt.Errorf("unknown tier")
	}
	for _, name := range remove {
		if role := b.findRole(s, name); role != nil {
			_ = s.GuildMemberRoleRemove(b.guildID, userID, role.ID)
		}
	}
	if role := b.findRole(s, add); role != nil {
		return s.GuildMemberRoleAdd(b.guildID, userID, role.ID)
	}
	return fmt.Errorf("role %s missing", add)
}

func (b *Bot) onJoin(s *discordgo.Session, ev *discordgo.GuildMemberAdd) {
	if ev.GuildID != b.guildID || ev.User == nil || ev.User.Bot {
		return
	}
	_ = b.exclusiveTier(s, ev.User.ID, membership.TierUser)
	age := accountAgeDays(ev.User)
	_, _ = b.members.Upsert(context.Background(), ev.User.ID, ev.User.Username, membership.TierUser, age, false)
	if ch := b.channel("welcome"); ch != "" {
		_, _ = s.ChannelMessageSendEmbed(ch, &discordgo.MessageEmbed{
			Title: "Welcome to Skillissue.ai", Color: 0x1ABC9C,
			Description: "<@" + ev.User.ID + ">\nRead the rules, then Verify to access the marketplace.",
			Footer:      &discordgo.MessageEmbedFooter{Text: disclaimer},
		})
	}
	b.audit(ev.GuildID, "member.join", "member "+ev.User.ID)
}

func (b *Bot) onLeave(s *discordgo.Session, ev *discordgo.GuildMemberRemove) {
	if ev.GuildID != b.guildID || ev.User == nil || ev.User.Bot {
		return
	}
	if ch := b.channel("goodbye"); ch != "" {
		_, _ = s.ChannelMessageSendEmbed(ch, &discordgo.MessageEmbed{Title: "A member left", Description: ev.User.Username + " left the server.", Color: 0x95A5A6})
	}
	b.audit(ev.GuildID, "member.leave", "member "+ev.User.ID)
}
