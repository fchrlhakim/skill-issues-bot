package discordbot

import (
	"errors"
	"net/http"

	"github.com/bwmarrin/discordgo"
)

// discordgo v0.29.0 predates these trigger types, so they are declared locally
// with the values from the Discord Auto Moderation reference. MEMBER_PROFILE
// checks the member profile rather than message content, which is the surface
// impersonation scams actually use.
const (
	autoModTriggerMentionSpam   discordgo.AutoModerationRuleTriggerType = 5
	autoModTriggerMemberProfile discordgo.AutoModerationRuleTriggerType = 6
)

// autoModSpec describes one additive AutoMod rule. Rules are created only when
// a rule with the same name does not already exist. Existing rules are never
// edited or deleted, matching the additive contract of syncLaunchRoles and
// syncLaunchGuild: a human stays the owner of what the server enforces.
type autoModSpec struct {
	Name        string
	TriggerType discordgo.AutoModerationRuleTriggerType
	Metadata    *discordgo.AutoModerationTriggerMetadata
	// BlockedMessage is shown to the member whose message was blocked.
	BlockedMessage string
}

// staffRoles are exempt from every rule so moderators can discuss and quote a
// scam while handling it.
var autoModStaffRoles = []string{
	"Platform Administrator",
	"Security & Fraud",
	"Operations Lead",
	"Moderator",
}

// autoModRules is the launch safety ruleset. Discord limits KEYWORD rules to 6
// per guild, MEMBER_PROFILE to 1, and MENTION_SPAM to 1; this stays inside all
// three. Keywords are capped at 60 characters and regexes at 260.
func autoModRules() []autoModSpec {
	return []autoModSpec{
		{
			Name:        "Safety · Credential Leak",
			TriggerType: discordgo.AutoModerationEventTriggerKeyword,
			Metadata: &discordgo.AutoModerationTriggerMetadata{
				KeywordFilter: []string{
					"seed phrase",
					"recovery phrase",
					"mnemonic phrase",
					"secret recovery phrase",
					"private key",
					"keystore json",
					"wallet.dat",
				},
				// A raw private key or a card number posted in public.
				RegexPatterns: []string{
					`0x[a-fA-F0-9]{64}`,
					`\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13})\b`,
				},
			},
			BlockedMessage: "Never post credentials here. Staff never ask for them. Use a ticket.",
		},
		{
			Name:        "Safety · Impersonation Profile",
			TriggerType: autoModTriggerMemberProfile,
			Metadata: &discordgo.AutoModerationTriggerMetadata{
				KeywordFilter: []string{
					"skill issues staff",
					"skill issues admin",
					"skill issues support",
					"skill issues official",
					"skillissue staff",
					"skillissue admin",
				},
			},
			BlockedMessage: "Your profile name may not claim to be Skill Issues staff.",
		},
		{
			Name:        "Safety · Invite Link",
			TriggerType: discordgo.AutoModerationEventTriggerKeyword,
			Metadata: &discordgo.AutoModerationTriggerMetadata{
				RegexPatterns: []string{
					`discord(?:\.gg|(?:app)?\.com/invite)/`,
				},
			},
			BlockedMessage: "External Discord invites are not allowed here.",
		},
		{
			Name:        "Safety · Mention Raid",
			TriggerType: autoModTriggerMentionSpam,
			Metadata: &discordgo.AutoModerationTriggerMetadata{
				MentionTotalLimit: 8,
			},
			BlockedMessage: "Too many mentions in one message.",
		},
	}
}

// autoModActions blocks the message and, when an internal log channel exists,
// also alerts staff there.
func (b *Bot) autoModActions(blockedMessage string) []discordgo.AutoModerationAction {
	actions := []discordgo.AutoModerationAction{{
		Type:     discordgo.AutoModerationRuleActionBlockMessage,
		Metadata: &discordgo.AutoModerationActionMetadata{CustomMessage: blockedMessage},
	}}
	alert := b.channel("audit-log")
	if alert == "" {
		alert = b.channel("staff-log")
	}
	if alert != "" {
		actions = append(actions, discordgo.AutoModerationAction{
			Type:     discordgo.AutoModerationRuleActionSendAlertMessage,
			Metadata: &discordgo.AutoModerationActionMetadata{ChannelID: alert},
		})
	}
	return actions
}

// staffRoleIDs resolves the exempt roles by name. Unknown names are skipped so
// a missing role never blocks rule creation.
func (b *Bot) staffRoleIDs(s *discordgo.Session) []string {
	roles, err := s.GuildRoles(b.guildID)
	if err != nil {
		return nil
	}
	want := map[string]bool{}
	for _, name := range autoModStaffRoles {
		want[name] = true
	}
	var ids []string
	for _, r := range roles {
		if want[r.Name] {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

// isMissingAccess reports whether Discord rejected the request with
// 403 Missing Access (code 50001). MEMBER_PROFILE rules return this even for a
// bot holding ADMINISTRATOR, so it must not abort the whole sync.
func isMissingAccess(err error) bool {
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) || rest.Response == nil {
		return false
	}
	if rest.Response.StatusCode != http.StatusForbidden {
		return false
	}
	return rest.Message == nil || rest.Message.Code == 50001
}

// syncAutoMod creates any missing launch safety rule. It never edits or deletes
// an existing rule, so a human's manual changes survive every run.
//
// unsupported names the rules Discord refused; the caller reports them instead
// of silently claiming success.
func (b *Bot) syncAutoMod(s *discordgo.Session) (created, skipped int, unsupported []string, err error) {
	existing, err := s.AutoModerationRules(b.guildID)
	if err != nil {
		return 0, 0, nil, err
	}
	have := map[string]bool{}
	for _, r := range existing {
		have[r.Name] = true
	}
	exemptRoles := b.staffRoleIDs(s)
	enabled := true
	for _, spec := range autoModRules() {
		if have[spec.Name] {
			skipped++
			continue
		}
		rule := &discordgo.AutoModerationRule{
			Name:            spec.Name,
			EventType:       discordgo.AutoModerationEventMessageSend,
			TriggerType:     spec.TriggerType,
			TriggerMetadata: spec.Metadata,
			Actions:         b.autoModActions(spec.BlockedMessage),
			Enabled:         &enabled,
		}
		if len(exemptRoles) > 0 {
			rule.ExemptRoles = &exemptRoles
		}
		if _, err := s.AutoModerationRuleCreate(b.guildID, rule); err != nil {
			if isMissingAccess(err) {
				unsupported = append(unsupported, spec.Name)
				continue
			}
			return created, skipped, unsupported, err
		}
		created++
		have[spec.Name] = true
	}
	return created, skipped, unsupported, nil
}
