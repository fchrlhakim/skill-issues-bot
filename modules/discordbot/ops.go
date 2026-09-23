package discordbot

import (
	"context"
	"time"

	"go-starter-kit/modules/ticket"
)

const (
	reminderInterval = 5 * time.Minute
	reminderAfter    = 30 * time.Minute
	reminderCooldown = 2 * time.Hour
	raidWindow       = time.Minute
	raidThreshold    = 8
	confirmTTL       = 120 * time.Second
)

type pendingConfirm struct {
	Kind      string
	TicketID  string
	ActorID   string
	MessageID string
	Reference string
	Expires   time.Time
}

func (b *Bot) startReminder() {
	ticker := time.NewTicker(reminderInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-b.stop:
				return
			case <-ticker.C:
				b.tickReminder()
			}
		}
	}()
}

func (b *Bot) tickReminder() {
	ctx := context.Background()
	open, err := b.tickets.ListOpen(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	if !b.reminderAt.IsZero() && now.Sub(b.reminderAt) < reminderCooldown {
		return
	}
	count := 0
	for _, rec := range open {
		if ticket.NeedsReminder(rec, now) {
			count++
		}
	}
	if count == 0 {
		return
	}
	b.reminderAt = now
	ch := b.channel("staff-log")
	if ch == "" {
		return
	}
	_, _ = b.session.ChannelMessageSend(ch,
		"Admin queue reminder: "+itoa(count)+" unassigned ticket(s) have been waiting at least 30 minutes. Use /tickets. This is a snapshot, not a response-time promise.")
}

func (b *Bot) noteJoin() {
	now := time.Now()
	b.joinsMu.Lock()
	b.recentJoins = append(b.recentJoins, now)
	cut := now.Add(-raidWindow)
	i := 0
	for i < len(b.recentJoins) && b.recentJoins[i].Before(cut) {
		i++
	}
	b.recentJoins = b.recentJoins[i:]
	burst := len(b.recentJoins)
	b.joinsMu.Unlock()
	if burst < raidThreshold {
		return
	}
	b.staff("RAID SIGNAL: " + itoa(burst) + " joins in the last minute (threshold " + itoa(raidThreshold) + "). Consider raising verification or pausing invites.")
	b.audit(b.guildID, "security.raid_signal", "joins="+itoa(burst))
}

func (b *Bot) putConfirm(key string, c pendingConfirm) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	if b.confirms == nil {
		b.confirms = map[string]pendingConfirm{}
	}
	now := time.Now()
	for k, v := range b.confirms {
		if now.After(v.Expires) {
			delete(b.confirms, k)
		}
	}
	if len(b.confirms) >= 1000 {
		for k := range b.confirms {
			delete(b.confirms, k)
			break
		}
	}
	b.confirms[key] = c
}

func (b *Bot) takeConfirm(key, messageID string) (pendingConfirm, bool) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	c, ok := b.confirms[key]
	if !ok || c.MessageID != messageID {
		return pendingConfirm{}, false
	}
	delete(b.confirms, key)
	if time.Now().After(c.Expires) {
		return pendingConfirm{}, false
	}
	return c, true
}

func confirmKey(actorID, ticketID string) string {
	return actorID + ":" + ticketID
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
