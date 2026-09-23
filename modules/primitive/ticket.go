package primitive

import (
	"time"

	"github.com/google/uuid"
)

type GuildMember struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DiscordID    string    `gorm:"column:discord_id;not null;uniqueIndex" json:"discord_id"`
	Username     string    `gorm:"not null" json:"username"`
	Tier         string    `gorm:"not null;index" json:"tier"`
	AccountAgeDays int     `gorm:"not null;default:0" json:"account_age_days"`
	Restricted   bool      `gorm:"not null;default:false" json:"restricted"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (GuildMember) TableName() string { return "guild_members" }

type Ticket struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	PublicID       string     `gorm:"column:public_id;not null;uniqueIndex" json:"public_id"`
	Seq            int        `gorm:"not null;index" json:"seq"`
	Type           string     `gorm:"not null;index" json:"type"`
	Code           string     `gorm:"not null" json:"code"`
	Region         string     `gorm:"not null;index" json:"region"`
	Workflow       string     `gorm:"not null" json:"workflow"`
	Status         string     `gorm:"not null;index" json:"status"`
	OpenerID       string     `gorm:"column:opener_id;not null;index" json:"opener_id"`
	OpenerTag      string     `gorm:"column:opener_tag" json:"opener_tag"`
	ThreadID       string     `gorm:"column:thread_id;not null;uniqueIndex" json:"thread_id"`
	AssignedTo     *string    `gorm:"column:assigned_to;index" json:"assigned_to,omitempty"`
	SummaryMsgID   string     `gorm:"column:summary_message_id" json:"summary_message_id"`
	OpenedAt       time.Time  `gorm:"column:opened_at;not null;index" json:"opened_at"`
	ClosedAt       *time.Time `gorm:"column:closed_at;index" json:"closed_at,omitempty"`
	DataJSON       string     `gorm:"column:data_json;type:jsonb;not null;default:'{}'" json:"-"`
	HistoryJSON    string     `gorm:"column:history_json;type:jsonb;not null;default:'[]'" json:"-"`
	ResolutionJSON string     `gorm:"column:resolution_json;type:jsonb" json:"-"`
	WithdrawalJSON string     `gorm:"column:withdrawal_json;type:jsonb" json:"-"`
	SellerJSON     string     `gorm:"column:seller_approval_json;type:jsonb" json:"-"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (Ticket) TableName() string { return "tickets" }

type TicketCounter struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	Seq       int       `gorm:"not null" json:"seq"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (TicketCounter) TableName() string { return "ticket_counters" }

type OutgoingMutation struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	PublicID    string    `gorm:"column:public_id;not null;uniqueIndex" json:"public_id"`
	TicketID    string    `gorm:"column:ticket_public_id;not null;index" json:"ticket_id"`
	SellerID    string    `gorm:"column:seller_id;not null;index" json:"seller_id"`
	AmountMinor int64     `gorm:"column:amount_minor;not null" json:"amount_minor"`
	Currency    string    `gorm:"not null" json:"currency"`
	Reference   string    `gorm:"not null;uniqueIndex" json:"reference"`
	By          string    `gorm:"not null" json:"by"`
	PaidAt      time.Time `gorm:"column:paid_at;not null" json:"paid_at"`
	CreatedAt   time.Time `json:"created_at"`
}

func (OutgoingMutation) TableName() string { return "outgoing_mutations" }
