package discordbot

import (
	"go-starter-kit/modules/membership"

	"github.com/bwmarrin/discordgo"
)

type roleSpec struct {
	Name    string
	Color   int
	Hoist   bool
	Mention bool
	Perms   int64
}

func launchRoles() []roleSpec {
	return []roleSpec{
		{Name: "Platform Administrator", Color: 0xD35400, Hoist: true, Perms: discordgo.PermissionManageServer | discordgo.PermissionManageRoles | discordgo.PermissionManageChannels | discordgo.PermissionViewAuditLogs | discordgo.PermissionManageMessages | discordgo.PermissionModerateMembers | discordgo.PermissionKickMembers | discordgo.PermissionBanMembers | discordgo.PermissionManageNicknames | discordgo.PermissionManageThreads},
		{Name: "Developer", Color: 0x7289DA, Hoist: true, Mention: true},
		{Name: "Operations Lead", Color: 0xE67E22, Hoist: true, Mention: true, Perms: discordgo.PermissionManageChannels | discordgo.PermissionManageMessages | discordgo.PermissionModerateMembers | discordgo.PermissionManageThreads | discordgo.PermissionViewAuditLogs},
		{Name: "Intermediary Lead", Color: 0xF1C40F, Hoist: true, Mention: true, Perms: discordgo.PermissionManageMessages | discordgo.PermissionModerateMembers | discordgo.PermissionManageThreads},
		{Name: "Intermediary", Color: 0xF39C12, Hoist: true, Mention: true},
		{Name: "Dispute & Compliance", Color: 0x9B59B6, Hoist: true, Mention: true},
		{Name: "Security & Fraud", Color: 0x2C3E50, Hoist: true, Mention: true, Perms: discordgo.PermissionManageMessages | discordgo.PermissionModerateMembers | discordgo.PermissionKickMembers | discordgo.PermissionBanMembers},
		{Name: "Finance & Risk", Color: 0x16A085, Mention: true},
		{Name: "Support & Verification", Color: 0x3498DB, Mention: true},
		{Name: "Regional & Translation", Color: 0x1ABC9C, Mention: true},
		{Name: "Moderator", Color: 0x5DADE2, Hoist: true, Mention: true, Perms: discordgo.PermissionManageMessages | discordgo.PermissionModerateMembers | discordgo.PermissionKickMembers | discordgo.PermissionManageNicknames},
		{Name: membership.RoleSeller, Color: 0x27AE60, Hoist: true},
		{Name: membership.RoleBuyer, Color: 0x2980B9},
		{Name: membership.RoleUser, Color: 0x95A5A6},
		{Name: "Under Review", Color: 0x4A4A4A},
		{Name: "Suspended", Color: 0x4A4A4A},
		{Name: "Blacklisted", Color: 0x4A4A4A},
		{Name: "English", Color: 0x00A8A8},
		{Name: "Bahasa Indonesia", Color: 0x00A8A8},
	}
}

func (b *Bot) syncLaunchRoles(s *discordgo.Session) (created int, err error) {
	existing, err := s.GuildRoles(b.guildID)
	if err != nil {
		return 0, err
	}
	have := map[string]bool{}
	for _, r := range existing {
		have[r.Name] = true
	}
	for _, spec := range launchRoles() {
		if have[spec.Name] || spec.Name == "Server Owner" {
			continue
		}
		params := &discordgo.RoleParams{Name: spec.Name, Color: &spec.Color, Hoist: &spec.Hoist, Mentionable: &spec.Mention, Permissions: &spec.Perms}
		if _, err := s.GuildRoleCreate(b.guildID, params); err != nil {
			return created, err
		}
		created++
		have[spec.Name] = true
	}
	return created, nil
}
