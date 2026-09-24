package discordbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/ticket"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type Bot struct {
	session     *discordgo.Session
	guildID     string
	tickets     ticket.ServiceInterface
	members     membership.ServiceInterface
	log         *logrus.Logger
	pending     sync.Map
	channelIDs  map[string]string
	writerLock  *processLock
	stop        chan struct{}
	joinsMu     sync.Mutex
	recentJoins []time.Time
	confirmMu   sync.Mutex
	confirms    map[string]pendingConfirm
	reminderAt  time.Time
	revenue     *http.Client
	revenueURL  string
	saas        *saasToken

	// guildCache memoizes the guild object. Every interaction needs its OwnerID,
	// and s.Guild() is a REST call, so without this each click costs a round trip
	// to Discord. Guarded by guildMu; refreshed at most once per guildCacheTTL.
	guildMu      sync.Mutex
	guildCache   *discordgo.Guild
	guildCacheAt time.Time

	// adminIDsCache memoizes the admin roster. Computing it walks every guild
	// member (up to 1000) and calls the permission resolver for each, which is
	// far too much work to repeat on every interaction; only opening a ticket or
	// handing one off actually needs the list.
	adminMu      sync.Mutex
	adminIDsList []string
	adminIDsAt   time.Time
}

func New(token, guildID string, tickets ticket.ServiceInterface, members membership.ServiceInterface, log *logrus.Logger) (*Bot, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers
	revenueURL := os.Getenv("SAAS_REVENUE_URL")
	revenueClient := &http.Client{Timeout: 5 * time.Second}
	bot := &Bot{session: session, guildID: guildID, tickets: tickets, members: members, log: log, channelIDs: map[string]string{}, stop: make(chan struct{}), confirms: map[string]pendingConfirm{}, revenue: revenueClient, revenueURL: revenueURL, saas: newSaaSToken(saasAuthURL(revenueURL), os.Getenv("SAAS_REVENUE_TOKEN"), os.Getenv("SAAS_REFRESH_TOKEN"), saasTokenFile(), revenueClient)}
	session.AddHandler(bot.onReady)
	session.AddHandler(bot.onInteraction)
	session.AddHandler(bot.onJoin)
	session.AddHandler(bot.onLeave)
	return bot, nil
}

func (b *Bot) Open() error {
	lease, err := acquireBotLock("data")
	if err != nil {
		return err
	}
	b.writerLock = lease
	writeHeartbeat("data")
	// Rotate before serving, so the persisted chain — not a possibly stale
	// environment value — is the credential in play. Rotating lazily would let
	// the two drift, and a restart would then replay a rotated-away token and
	// revoke the whole family.
	b.primeSaaSToken()
	if err := b.session.Open(); err != nil {
		lease.release()
		return err
	}
	_, err = b.session.ApplicationCommandBulkOverwrite(b.session.State.User.ID, b.guildID, slashCommands())
	if err != nil {
		return err
	}
	b.startReminder()
	return nil
}

// primeSaaSToken rotates the SaaS credential once at startup. A failure is
// logged, not fatal: the gateway must still come up, and /revenue degrades to
// "unavailable" on its own.
func (b *Bot) primeSaaSToken() {
	if b.saas == nil || !b.saas.canRotate() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := b.saas.Access(ctx); err != nil {
		b.log.WithError(err).Warn("could not prime the saas credential; /revenue may be unavailable")
	}
}

func (b *Bot) Close() error {
	select {
	case <-b.stop:
	default:
		close(b.stop)
	}
	if b.writerLock != nil {
		b.writerLock.release()
	}
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
func (b *Bot) serverSnapshot(ctx context.Context, s *discordgo.Session) string {
	members, err := b.members.Snapshot(ctx)
	if err != nil {
		return err.Error()
	}
	tickets, err := b.tickets.Snapshot(ctx)
	if err != nil {
		return err.Error()
	}
	discordCount := "unavailable"
	if guild, err := s.GuildWithCounts(b.guildID); err == nil {
		discordCount = itoa(guild.ApproximateMemberCount)
	}
	return strings.Join([]string{
		"Discord members: " + discordCount,
		"Stored members: " + itoa(members.Members),
		"User / Buyer / Seller: " + itoa(members.Tiers[membership.TierUser]) + " / " + itoa(members.Tiers[membership.TierBuyer]) + " / " + itoa(members.Tiers[membership.TierSeller]),
		"Restricted: " + itoa(members.Restricted),
		"Open tickets: " + itoa(tickets.OpenTickets),
		"Tickets by status: " + countList(tickets.Tickets),
	}, "\n")
}

func (b *Bot) revenueSnapshot(ctx context.Context) string {
	snapshot, err := b.tickets.Snapshot(ctx)
	if err != nil {
		return err.Error()
	}
	lines := []string{"Bot confirmed outgoing records: " + itoa(snapshot.Payments)}
	if snapshot.Payments == 0 {
		lines = append(lines, "No confirmed bot payments. Approval is not payment.")
	}
	for _, currency := range sortedKeys(snapshot.Totals) {
		lines = append(lines, currency+": "+ticket.FormatAmount(snapshot.Totals[currency], currency))
	}
	saas, err := b.saasRevenue(ctx)
	if err != nil {
		// A silent failure here costs hours: a 401, a network error, and a bad
		// body all render the same line. Log the cause; never log the token.
		b.log.WithError(err).Warn("saas revenue lookup failed")
		lines = append(lines, "SaaS revenue: unavailable")
	} else {
		lines = append(lines, saas)
	}
	return strings.Join(lines, "\n")
}

type saasFinance struct {
	Data struct {
		Summary struct {
			TodayFee   string `json:"today_platform_fee_nano_usd"`
			MonthFee   string `json:"month_platform_fee_nano_usd"`
			AllFee     string `json:"all_time_platform_fee_nano_usd"`
			TodayCount int64  `json:"today_requests"`
			MonthCount int64  `json:"month_requests"`
			AllCount   int64  `json:"all_time_requests"`
		} `json:"summary"`
	} `json:"data"`
}

func (b *Bot) saasRevenue(ctx context.Context) (string, error) {
	if b.revenueURL == "" || b.saas == nil {
		return "", fmt.Errorf("not configured")
	}
	body, err := b.saasGet(ctx)
	if err != nil {
		return "", err
	}
	var payload saasFinance
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	summary := payload.Data.Summary
	return strings.Join([]string{
		"SaaS platform fee today: " + nanoUSD(summary.TodayFee) + " (" + itoa64(summary.TodayCount) + " requests)",
		"SaaS platform fee this month: " + nanoUSD(summary.MonthFee) + " (" + itoa64(summary.MonthCount) + " requests)",
		"SaaS platform fee all time: " + nanoUSD(summary.AllFee) + " (" + itoa64(summary.AllCount) + " requests)",
	}, "\n"), nil
}

// saasGet fetches the finance overview, rotating once if the cached access token
// was rejected. A 401 is the only signal that the token died early, so the retry
// is deliberately narrow: a second failure is reported, not looped on.
func (b *Bot) saasGet(ctx context.Context) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		token, err := b.saas.Access(ctx)
		if err != nil {
			return nil, err
		}
		body, status, err := b.saasFetch(ctx, token)
		if err != nil {
			return nil, err
		}
		if status == http.StatusOK {
			return body, nil
		}
		if status == http.StatusUnauthorized && attempt == 0 && b.saas.canRotate() {
			b.log.Warn("saas revenue token rejected; rotating once")
			b.saas.Invalidate()
			continue
		}
		return nil, fmt.Errorf("status %d from %s", status, b.revenueURL)
	}
}

func (b *Bot) saasFetch(ctx context.Context, token string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.revenueURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := b.revenue.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, 0, err
	}
	return body, res.StatusCode, nil
}

func nanoUSD(value string) string {
	var nano int64
	fmt.Sscan(value, &nano)
	sign := ""
	if nano < 0 {
		sign = "-"
		nano = -nano
	}
	return sign + "$" + itoa64(nano/1_000_000_000) + "." + fmt.Sprintf("%09d", nano%1_000_000_000)
}

func itoa64(value int64) string { return itoa(int(value)) }

func countList(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(counts))
	for _, key := range sortedKeys(counts) {
		parts = append(parts, key+" "+itoa(counts[key]))
	}
	return strings.Join(parts, ", ")
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
	guild := b.guild(s)
	if guild == nil {
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

// guildCacheTTL bounds how stale a cached guild object may be. It is long enough
// to remove a REST call per interaction and short enough that a rename or an
// ownership change is picked up without a restart.
const guildCacheTTL = 2 * time.Minute

// guild returns the guild, from cache when fresh. A cache miss that fails is not
// cached, so a transient REST error does not pin an empty guild for the TTL.
func (b *Bot) guild(s *discordgo.Session) *discordgo.Guild {
	b.guildMu.Lock()
	defer b.guildMu.Unlock()
	if b.guildCache != nil && time.Since(b.guildCacheAt) < guildCacheTTL {
		return b.guildCache
	}
	guild, err := s.Guild(b.guildID)
	if err != nil {
		return b.guildCache
	}
	b.guildCache, b.guildCacheAt = guild, time.Now()
	return guild
}

// actor builds the ticket actor for one interaction. Only the fields every
// caller needs are computed eagerly; AvailableAdmins is resolved lazily by
// actorWithAdmins because computing it walks the whole member list.
func (b *Bot) actor(s *discordgo.Session, m *discordgo.Member) ticket.Actor {
	username, tag := "", ""
	id := ""
	if m != nil && m.User != nil {
		id, username, tag = m.User.ID, m.User.Username, m.User.String()
	}
	owner := ""
	if guild := b.guild(s); guild != nil {
		owner = guild.OwnerID
	}
	return ticket.Actor{
		ID: id, Username: username, Tag: tag, Roles: b.roleNames(s, m),
		IsAdmin: b.isAdmin(s, m), OwnerID: owner,
	}
}

// withAdmins fills AvailableAdmins for the paths that actually hand a ticket to
// an admin (open, handoff). Those are rare; every other interaction would pay
// for the member walk and never read it.
func (b *Bot) withAdmins(s *discordgo.Session, actor ticket.Actor) ticket.Actor {
	actor.AvailableAdmins = b.adminIDs(s)
	return actor
}

func (b *Bot) adminIDs(s *discordgo.Session) []string {
	b.adminMu.Lock()
	defer b.adminMu.Unlock()
	if b.adminIDsList != nil && time.Since(b.adminIDsAt) < guildCacheTTL {
		return b.adminIDsList
	}
	guild := b.guild(s)
	if guild == nil {
		return b.adminIDsList
	}
	ids := []string{guild.OwnerID}
	members, err := s.GuildMembers(b.guildID, "", 1000)
	if err != nil {
		return b.adminIDsList
	}
	seen := map[string]bool{guild.OwnerID: true}
	for _, member := range members {
		if member.User == nil || member.User.Bot {
			continue
		}
		if b.isAdmin(s, member) && !seen[member.User.ID] {
			seen[member.User.ID] = true
			ids = append(ids, member.User.ID)
		}
	}
	b.adminIDsList, b.adminIDsAt = ids, time.Now()
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
	// One call so the role and the stored tier cannot disagree. A failure here
	// leaves the member without the User role while the row would still say
	// TierUser, so the two sources of truth silently diverge.
	if err := b.setTier(s, ev.User.ID, membership.TierUser, ev.User.Username); err != nil && b.log != nil {
		b.log.WithError(err).WithField("discord_id", ev.User.ID).Warn("could not grant the User tier on join")
	}
	b.noteJoin()
	age := accountAgeDays(ev.User)
	if age < 7 {
		b.staff("New account joined: member " + ev.User.ID + " is " + itoa(age) + "d old. Watch first messages.")
	}
	if ch := b.channel("welcome"); ch != "" {
		embed := &discordgo.MessageEmbed{
			Title: "Welcome to Skillissue.ai", Color: 0x1ABC9C,
			Description: "<@" + ev.User.ID + ">\nRead the rules, then Verify to access the marketplace.\n" + SafetyCopy,
			Footer:      &discordgo.MessageEmbedFooter{Text: disclaimer},
		}
		png := renderWelcomePNG(ev.User.Username, 0)
		_, _ = s.ChannelMessageSendComplex(ch, &discordgo.MessageSend{
			Content: "<@" + ev.User.ID + ">", Embeds: []*discordgo.MessageEmbed{embed},
			Files:           []*discordgo.File{{Name: "welcome.png", Reader: bytes.NewReader(png)}},
			AllowedMentions: &discordgo.MessageAllowedMentions{Users: []string{ev.User.ID}},
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
