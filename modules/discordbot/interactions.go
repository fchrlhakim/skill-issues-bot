package discordbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/ticket"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) onInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID != b.guildID {
		b.ephemeral(s, i, "Use this command in the configured server.")
		return
	}
	data := i.Interaction
	if data.Type == discordgo.InteractionMessageComponent {
		cid := data.MessageComponentData().CustomID
		if cid == "ticket:open" || strings.HasPrefix(cid, "ticket:request:") {
			key := ""
			if cid == "ticket:open" {
				vals := data.MessageComponentData().Values
				if len(vals) == 1 {
					key = vals[0]
				}
			} else {
				key = strings.TrimPrefix(cid, "ticket:request:")
			}
			t, ok := ticket.TypeByKey(key)
			if !ok {
				b.ephemeral(s, i, "Unknown ticket type.")
				return
			}
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseModal, Data: ticketModal(t)})
			return
		}
	}
	if data.Type == discordgo.InteractionApplicationCommand && data.ApplicationCommandData().Name == "withdraw" {
		t, _ := ticket.TypeByKey("withdraw")
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{Type: discordgo.InteractionResponseModal, Data: ticketModal(t)})
		return
	}
	if err := b.deferEphemeral(s, i); err != nil {
		return
	}
	member := i.Member
	if member == nil || member.User == nil {
		b.edit(s, i, "Current human server membership could not be verified.", nil, nil)
		return
	}
	actor := b.actor(s, member)
	admin := membership.IsTicketAdmin(actor.ID, actor.OwnerID, false, actor.IsAdmin)

	switch data.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i, actor, admin)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(s, i, actor, admin)
	case discordgo.InteractionModalSubmit:
		b.handleModal(s, i, actor)
	}
}

func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate, actor ticket.Actor, admin bool) {
	name := i.ApplicationCommandData().Name
	if adminOnlyNames[name] && !admin {
		b.edit(s, i, "Admin only.", nil, nil)
		return
	}
	ctx := context.Background()
	option := func(n string) string {
		for _, o := range i.ApplicationCommandData().Options {
			if o.Name == n {
				if o.Type == discordgo.ApplicationCommandOptionString {
					return o.StringValue()
				}
				if o.Type == discordgo.ApplicationCommandOptionUser {
					return o.UserValue(s).ID
				}
			}
		}
		return ""
	}
	switch name {
	case "guildsync":
		cats, chans, err := b.syncLaunchGuild(s)
		if err != nil {
			b.edit(s, i, err.Error(), nil, nil)
			return
		}
		b.edit(s, i, "Guild layout synced. Created "+itoa(cats)+" categories and "+itoa(chans)+" channels. Nothing was deleted or overwritten.", nil, nil)
	case "automodsync":
		created, skipped, unsupported, err := b.syncAutoMod(s)
		if err != nil {
			b.edit(s, i, err.Error(), nil, nil)
			return
		}
		msg := "AutoMod synced. Created " + itoa(created) + " rules, " + itoa(skipped) + " already existed."
		if len(unsupported) > 0 {
			msg += " Discord refused " + itoa(len(unsupported)) + " rule(s): " + strings.Join(unsupported, ", ") +
				". Those must be created by hand in Server Settings."
		}
		msg += " Nothing was edited or deleted."
		b.edit(s, i, msg, nil, nil)
	case "rolesync":
		n, err := b.syncLaunchRoles(s)
		if err != nil {
			b.edit(s, i, err.Error(), nil, nil)
			return
		}
		b.edit(s, i, "Launch roles synced. Created "+itoa(n)+" missing roles. Nothing was deleted.", nil, nil)
	case "panel":
		_, _ = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{Embeds: ticketPanel().Embeds, Components: ticketPanel().Components})
		b.edit(s, i, "Panel posted.", nil, nil)
	case "rolepanel":
		_, _ = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{Embeds: languagePicker().Embeds, Components: languagePicker().Components})
		_, _ = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{Embeds: regionPicker().Embeds, Components: regionPicker().Components})
		b.edit(s, i, "Role pickers posted.", nil, nil)
	case "verifypanel":
		_, _ = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{Embeds: verifyPanel().Embeds, Components: verifyPanel().Components})
		b.edit(s, i, "Panel posted.", nil, nil)
	case "safety":
		b.edit(s, i, SafetyCopy, nil, nil)
	case "whoami":
		info := b.tickets.Whoami(actor)
		b.edit(s, i, fmt.Sprintf("tier=%v admin=%v restricted=%v", info["tier"], info["admin"], info["restricted"]), nil, nil)
	case "tickets":
		recs, err := b.tickets.Queue(ctx, actor, 1)
		if err != nil {
			b.edit(s, i, err.Error(), nil, nil)
			return
		}
		b.edit(s, i, formatQueue(recs), nil, nil)
	case "mutasi":
		rows, err := b.tickets.Mutasi(ctx, actor)
		if err != nil {
			b.edit(s, i, err.Error(), nil, nil)
			return
		}
		if len(rows) == 0 {
			b.edit(s, i, "No outgoing records.", nil, nil)
			return
		}
		var lines []string
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("`%s` %s %s %s", row.PublicID, row.Reference, row.Currency, row.PaidAt.Format("2006-01-02")))
		}
		b.edit(s, i, strings.Join(lines, "\n"), nil, nil)
	case "memberpanel":
		// Posts once, then keeps editing that same message, so the counts in the
		// channel are always the live ones and no stale copy is left behind.
		note, err := b.publishMemberPanel(ctx)
		if err != nil {
			b.edit(s, i, "Could not publish the member panel: "+err.Error(), nil, nil)
			return
		}
		b.edit(s, i, note, nil, nil)
	case "areapanel":
		side := option("side")
		if side != membership.TierSeller && side != membership.TierBuyer {
			b.edit(s, i, "Choose either the Seller Area or the Buyer Area.", nil, nil)
			return
		}
		panel := areaPanel(side)
		_, _ = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
			Embeds: panel.Embeds, Components: panel.Components, AllowedMentions: &discordgo.MessageAllowedMentions{},
		})
		b.edit(s, i, "Panel posted.", nil, nil)
	case "membersync":
		repaired, pruned, err := b.reconcileTiers(ctx)
		if err != nil {
			b.edit(s, i, "Sync failed: "+err.Error(), nil, nil)
			return
		}
		b.refreshMemberPanel(ctx)
		b.edit(s, i, b.reconcileSummary(repaired, pruned), nil, nil)
	case "server":
		b.edit(s, i, b.serverSnapshot(ctx, s), nil, nil)
	case "revenue":
		b.edit(s, i, b.revenueSnapshot(ctx), nil, nil)
	case "lookup":
		targetID := option("member")
		m, err := s.GuildMember(b.guildID, targetID)
		if err != nil {
			b.edit(s, i, "That user is not in this server.", nil, nil)
			return
		}
		age := accountAgeDays(m.User)
		b.edit(s, i, fmt.Sprintf("member %s · age %dd · roles %s", m.User.Username, age, strings.Join(b.roleNames(s, m), ", ")), nil, nil)
	case "testwelcome":
		b.edit(s, i, "Welcome preview: read the rules, then Verify. "+disclaimer, nil, nil)
	case "ticket-status", "ticket-handoff", "ticket-resolution", "seller-approve", "withdraw-paid":
		rec, err := b.tickets.GetByThread(ctx, i.ChannelID)
		if err != nil {
			b.edit(s, i, "Not a ticket thread.", nil, nil)
			return
		}
		if rec.OpenerID == actor.ID && name != "ticket-status" {
			b.edit(s, i, "You opened this ticket. Another admin must handle your case.", nil, nil)
			return
		}
		switch name {
		case "ticket-status":
			if !admin && rec.OpenerID != actor.ID {
				b.edit(s, i, "Only this ticket's opener or an admin can view its record.", nil, nil)
				return
			}
			b.edit(s, i, "", []*discordgo.MessageEmbed{ticketSummary(rec)}, nil)
		case "ticket-handoff":
			next, err := b.tickets.Handoff(ctx, b.withAdmins(s, actor), rec.ID, option("admin"), option("reason"))
			b.replyRecord(s, i, next, err)
		case "ticket-resolution":
			next, err := b.tickets.SetResolution(ctx, actor, rec.ID, option("note"))
			b.replyRecord(s, i, next, err)
		case "seller-approve":
			opener, _ := s.GuildMember(b.guildID, rec.OpenerID)
			next, err := b.tickets.ApproveSeller(ctx, actor, rec.ID, option("reason"), b.roleNames(s, opener))
			if err == nil {
				// The record now says Seller. If Discord never got the role the member
				// is a seller in the database and a plain member in the guild, so a
				// silent failure here is a real divergence between the two stores.
				if tierErr := b.setTier(s, rec.OpenerID, membership.TierSeller, ""); tierErr != nil && b.log != nil {
					b.log.WithError(tierErr).WithField("discord_id", rec.OpenerID).Warn("seller approved but the Seller role or the stored tier did not update")
				} else {
					b.refreshMemberPanel(ctx)
				}
			}
			b.replyRecord(s, i, next, err)
		case "withdraw-paid":
			prompt := paidConfirm(rec, option("reference"))
			msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: prompt.Content, Embeds: prompt.Embeds, Components: prompt.Components, Flags: discordgo.MessageFlagsEphemeral,
			})
			if err != nil || msg == nil {
				b.edit(s, i, "Payment not saved. Confirmation could not be posted.", nil, nil)
				return
			}
			b.putConfirm(confirmKey(actor.ID, rec.ID), pendingConfirm{Kind: "paid", TicketID: rec.ID, ActorID: actor.ID, MessageID: msg.ID, Reference: option("reference"), Expires: time.Now().Add(confirmTTL)})
			return
		}
	default:
		b.edit(s, i, "Unknown command.", nil, nil)
	}
}

func (b *Bot) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, actor ticket.Actor, admin bool) {
	cid := i.MessageComponentData().CustomID
	ctx := context.Background()
	if strings.HasPrefix(cid, "rolepick:") {
		role, ok := membership.ResolvePickedRole(cid)
		if !ok {
			b.edit(s, i, "That option is not available.", nil, nil)
			return
		}
		found := b.findRole(s, role)
		if found == nil {
			b.edit(s, i, "Cannot assign **"+role+"**: role does not exist on this server yet.", nil, nil)
			return
		}
		has := false
		for _, id := range i.Member.Roles {
			if id == found.ID {
				has = true
			}
		}
		if has {
			_ = s.GuildMemberRoleRemove(b.guildID, actor.ID, found.ID)
			b.edit(s, i, "Removed **"+role+"**", nil, nil)
			return
		}
		if err := s.GuildMemberRoleAdd(b.guildID, actor.ID, found.ID); err != nil {
			b.edit(s, i, "Cannot assign **"+role+"**.", nil, nil)
			return
		}
		b.edit(s, i, "Added **"+role+"**", nil, nil)
		return
	}
	if cid == membership.VerifyButtonID {
		decision, err := b.members.Verify(ctx, actor.ID, actor.Roles, accountAgeDays(i.Member.User))
		if err != nil || (decision.Action != "verify" && decision.Action != "already") {
			msg := decision.Reason
			if msg == "" {
				msg = "Verification was not confirmed. Ask an admin to review your membership."
			}
			b.edit(s, i, msg, nil, nil)
			return
		}
		if decision.Action == "verify" {
			if tierErr := b.setTier(s, actor.ID, membership.TierBuyer, i.Member.User.Username); tierErr != nil && b.log != nil {
				b.log.WithError(tierErr).WithField("discord_id", actor.ID).Warn("verify granted but the Buyer role or the stored tier did not update")
			}
			// Say exactly what changed and what did not. A new member has no way
			// to tell whether they are a Buyer or a Seller, and the two areas
			// look identical until one of them opens.
			b.refreshMemberPanel(ctx)
			b.edit(s, i, "You are now a **Buyer**. The Buyer Area is open to you; "+
				"the Seller Area stays hidden until an admin approves a seller application.", nil, nil)
			return
		}
		b.edit(s, i, "You are already a **"+tierLabel(actor.Roles)+"**. Your marketplace access is unchanged.", nil, nil)
		return
	}
	parts := strings.SplitN(cid, ":", 3)
	if len(parts) != 3 || parts[0] != "ticket" {
		b.edit(s, i, "That control is no longer active.", nil, nil)
		return
	}
	action, id := parts[1], parts[2]
	rec, err := b.tickets.Get(ctx, id)
	if err != nil || rec.ThreadID != i.ChannelID {
		b.edit(s, i, "This control does not belong to this ticket thread.", nil, nil)
		return
	}
	if !admin {
		b.edit(s, i, "Admin only.", nil, nil)
		return
	}
	if rec.OpenerID == actor.ID {
		b.edit(s, i, "You opened this ticket. Another admin must handle your case.", nil, nil)
		return
	}
	if action == "confirmclose" || action == "cancelclose" || action == "confirmpaid" || action == "cancelpaid" {
		prompt, ok := b.takeConfirm(confirmKey(actor.ID, rec.ID), i.Message.ID)
		if !ok {
			b.edit(s, i, "This confirmation is no longer current. Use the latest prompt.", nil, nil)
			return
		}
		if action == "cancelclose" || action == "cancelpaid" {
			b.edit(s, i, "Cancelled. The ticket is unchanged.", nil, nil)
			return
		}
		if action == "confirmclose" {
			next, err := b.tickets.Transition(ctx, actor, rec.ID, "closed")
			b.replyRecord(s, i, next, err)
			return
		}
		next, err := b.tickets.RecordPayment(ctx, actor, rec.ID, prompt.Reference)
		b.replyRecord(s, i, next, err)
		return
	}
	switch action {
	case "claim":
		next, err := b.tickets.Claim(ctx, actor, rec.ID)
		b.replyRecord(s, i, next, err)
	case "status":
		next := ticket.NextStatuses(rec.Type, rec.Workflow, rec.Status)
		if len(next) == 0 {
			b.edit(s, i, "No transitions remain.", nil, nil)
			return
		}
		opts := make([]discordgo.SelectMenuOption, 0, len(next))
		for _, st := range next {
			opts = append(opts, discordgo.SelectMenuOption{Label: st.Label, Value: st.Key})
		}
		b.edit(s, i, "Current status: **"+rec.Status+"**. Only legal transitions are shown.", nil, []discordgo.MessageComponent{
			discordgo.ActionsRow{Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{CustomID: "ticket:setstatus:" + rec.ID, Placeholder: "New status…", Options: opts},
			}},
		})
	case "setstatus":
		vals := i.MessageComponentData().Values
		if len(vals) != 1 {
			b.edit(s, i, "That transition is no longer available.", nil, nil)
			return
		}
		to := vals[0]
		if to == "closed" {
			next, err := b.tickets.Transition(ctx, actor, rec.ID, "closed")
			b.replyRecord(s, i, next, err)
			return
		}
		if ticket.IsWithdrawal(rec.Type, rec.Workflow) && to == "approved" {
			next, err := b.tickets.ApproveWithdrawal(ctx, actor, rec.ID)
			b.replyRecord(s, i, next, err)
			return
		}
		next, err := b.tickets.Transition(ctx, actor, rec.ID, to)
		b.replyRecord(s, i, next, err)
	case "close":
		prompt := closeConfirm(rec)
		msg, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: prompt.Content, Embeds: prompt.Embeds, Components: prompt.Components, Flags: discordgo.MessageFlagsEphemeral,
		})
		if err != nil || msg == nil {
			b.edit(s, i, "Close confirmation could not be posted.", nil, nil)
			return
		}
		b.putConfirm(confirmKey(actor.ID, rec.ID), pendingConfirm{Kind: "close", TicketID: rec.ID, ActorID: actor.ID, MessageID: msg.ID, Expires: time.Now().Add(confirmTTL)})
		return
	default:
		b.edit(s, i, "That control is no longer active.", nil, nil)
	}
}

func (b *Bot) handleModal(s *discordgo.Session, i *discordgo.InteractionCreate, actor ticket.Actor) {
	cid := i.ModalSubmitData().CustomID
	key := strings.TrimPrefix(cid, "ticket:modal:")
	spec, ok := ticket.TypeByKey(key)
	if !ok {
		b.edit(s, i, "Unknown ticket type.", nil, nil)
		return
	}
	data := map[string]string{}
	for _, row := range i.ModalSubmitData().Components {
		var comps []discordgo.MessageComponent
		switch typed := row.(type) {
		case discordgo.ActionsRow:
			comps = typed.Components
		case *discordgo.ActionsRow:
			comps = typed.Components
		}
		for _, comp := range comps {
			switch input := comp.(type) {
			case discordgo.TextInput:
				data[input.CustomID] = input.Value
			case *discordgo.TextInput:
				data[input.CustomID] = input.Value
			}
		}
	}
	parent := b.channel("open-a-ticket")
	if parent == "" {
		parent = i.ChannelID
	}
	thread, err := s.ThreadStartComplex(parent, &discordgo.ThreadStart{
		Name: "ticket", Type: discordgo.ChannelTypeGuildPrivateThread, Invitable: false, AutoArchiveDuration: 10080,
	})
	if err != nil {
		b.edit(s, i, "Could not create the ticket thread. Ask an admin to check permissions.", nil, nil)
		return
	}
	actor = b.withAdmins(s, actor)
	rec, err := b.tickets.Create(context.Background(), actor, ticket.CreateInput{Type: spec.Key, Data: data, ThreadID: thread.ID})
	if err != nil {
		_, _ = s.ChannelDelete(thread.ID)
		b.edit(s, i, err.Error(), nil, nil)
		return
	}
	_, _ = s.ChannelEdit(thread.ID, &discordgo.ChannelEdit{Name: rec.ID})
	_ = s.ThreadMemberAdd(thread.ID, actor.ID)
	for _, adminID := range actor.AvailableAdmins {
		if adminID != actor.ID {
			_ = s.ThreadMemberAdd(thread.ID, adminID)
		}
	}
	_, _ = s.ChannelMessageSendComplex(thread.ID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{ticketSummary(rec)}, Components: ticketControls(rec),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	b.edit(s, i, "Ticket **"+rec.ID+"** opened in <#"+thread.ID+">.", nil, nil)
	b.audit(i.GuildID, "ticket.create", rec.ID+" by "+actor.ID)
}

func (b *Bot) replyRecord(s *discordgo.Session, i *discordgo.InteractionCreate, rec ticket.Record, err error) {
	if err != nil {
		b.edit(s, i, err.Error(), nil, nil)
		return
	}
	if rec.SummaryMsgID != "" {
		embeds := []*discordgo.MessageEmbed{ticketSummary(rec)}
		comps := ticketControls(rec)
		_, _ = s.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID: rec.SummaryMsgID, Channel: rec.ThreadID, Embeds: &embeds, Components: &comps,
		})
	}
	b.edit(s, i, "Saved **"+rec.ID+"** · "+rec.Status, nil, nil)
}

func accountAgeDays(user *discordgo.User) int {
	if user == nil {
		return 0
	}
	created, err := discordgo.SnowflakeTimestamp(user.ID)
	if err != nil {
		return 0
	}
	return int(time.Since(created).Hours() / 24)
}
