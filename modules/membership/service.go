package membership

import (
	"context"
	"errors"
	"time"

	"go-starter-kit/modules/primitive"

	"gorm.io/gorm"
)

type ServiceInterface interface {
	Upsert(ctx context.Context, discordID, username, tier string, ageDays int, restricted bool) (primitive.GuildMember, error)
	Find(ctx context.Context, discordID string) (primitive.GuildMember, error)
	Verify(ctx context.Context, discordID string, roles []string, ageDays int) (VerifyDecision, error)
	Snapshot(ctx context.Context) (ServerSnapshot, error)
}

type RepositoryInterface interface {
	Upsert(ctx context.Context, member primitive.GuildMember) (primitive.GuildMember, error)
	FindByDiscordID(ctx context.Context, discordID string) (primitive.GuildMember, error)
	Snapshot(ctx context.Context) (ServerSnapshot, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) Upsert(ctx context.Context, member primitive.GuildMember) (primitive.GuildMember, error) {
	var existing primitive.GuildMember
	err := r.db.WithContext(ctx).Where("discord_id = ?", member.DiscordID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(&member).Error; err != nil {
			return primitive.GuildMember{}, err
		}
		return member, nil
	}
	if err != nil {
		return primitive.GuildMember{}, err
	}
	existing.Username = member.Username
	existing.Tier = member.Tier
	existing.AccountAgeDays = member.AccountAgeDays
	existing.Restricted = member.Restricted
	existing.UpdatedAt = time.Now().UTC()
	if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return primitive.GuildMember{}, err
	}
	return existing, nil
}

func (r *Repository) FindByDiscordID(ctx context.Context, discordID string) (primitive.GuildMember, error) {
	var member primitive.GuildMember
	if err := r.db.WithContext(ctx).Where("discord_id = ?", discordID).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return primitive.GuildMember{}, err
		}
		return primitive.GuildMember{}, err
	}
	return member, nil
}

type ServerSnapshot struct {
	Members    int
	Tiers      map[string]int
	Restricted int
}

func (r *Repository) Snapshot(ctx context.Context) (ServerSnapshot, error) {
	var members []primitive.GuildMember
	if err := r.db.WithContext(ctx).Select("tier", "restricted").Find(&members).Error; err != nil {
		return ServerSnapshot{}, err
	}
	out := ServerSnapshot{Members: len(members), Tiers: map[string]int{}}
	for _, member := range members {
		out.Tiers[member.Tier]++
		if member.Restricted {
			out.Restricted++
		}
	}
	return out, nil
}

type Service struct {
	repository RepositoryInterface
}

func NewService(repository RepositoryInterface) ServiceInterface {
	return &Service{repository: repository}
}

func (s *Service) Upsert(ctx context.Context, discordID, username, tier string, ageDays int, restricted bool) (primitive.GuildMember, error) {
	if tier == "" {
		tier = TierNone
	}
	return s.repository.Upsert(ctx, primitive.GuildMember{
		DiscordID: discordID, Username: username, Tier: tier, AccountAgeDays: ageDays, Restricted: restricted,
	})
}

func (s *Service) Find(ctx context.Context, discordID string) (primitive.GuildMember, error) {
	return s.repository.FindByDiscordID(ctx, discordID)
}

func (s *Service) Verify(ctx context.Context, discordID string, roles []string, ageDays int) (VerifyDecision, error) {
	decision := DecideVerification(roles, ageDays, true, true)
	if decision.Action != "verify" && decision.Action != "already" {
		return decision, nil
	}
	tier := MemberTier(roles)
	if decision.Action == "verify" {
		tier = TierBuyer
	}
	_, err := s.Upsert(ctx, discordID, "", tier, ageDays, Restricted(roles))
	return decision, err
}
func (s *Service) Snapshot(ctx context.Context) (ServerSnapshot, error) {
	return s.repository.Snapshot(ctx)
}
