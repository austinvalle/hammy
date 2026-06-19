package dad

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/bwmarrin/discordgo"
)

// addDadCommand assigns the dad role to a mentioned user (!adddad @user).
type addDadCommand struct {
	logger    *slog.Logger
	dadRoleID string
}

func NewAddDadCommand(l *slog.Logger, dadRoleID string) *addDadCommand {
	return &addDadCommand{logger: l, dadRoleID: dadRoleID}
}

func (c *addDadCommand) Name() string { return "adddad" }

func (c *addDadCommand) CanActivate(_ *discordgo.Session, m discordgo.Message) bool {
	return len(m.Mentions) == 1 && len(m.Content) >= 8 && m.Content[:8] == "!adddad "
}

func (c *addDadCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	if len(m.Mentions) != 1 {
		return &discordgo.MessageSend{Content: "Usage: !adddad @user"}, nil
	}

	target := m.Mentions[0]
	if err := s.GuildMemberRoleAdd(m.GuildID, target.ID, c.dadRoleID); err != nil {
		return nil, fmt.Errorf("error adding dad role: %w", err)
	}

	return &discordgo.MessageSend{
		Content: fmt.Sprintf("✅ <@%s> has been added to the dad pool!", target.ID),
	}, nil
}

// setDadCommand overrides dad of the month for current + next month (!setdad @user, admin only).
type setDadCommand struct {
	logger    *slog.Logger
	repo      Repository
	channelID string
	dadRoleID string
}

func NewSetDadCommand(l *slog.Logger, repo Repository, channelID, dadRoleID string) *setDadCommand {
	return &setDadCommand{logger: l, repo: repo, channelID: channelID, dadRoleID: dadRoleID}
}

func (c *setDadCommand) Name() string { return "setdad" }

func (c *setDadCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	return isAdmin(s, m) && len(m.Mentions) == 1 && len(m.Content) >= 8 && m.Content[:8] == "!setdad "
}

func (c *setDadCommand) Handler(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	if len(m.Mentions) != 1 {
		return &discordgo.MessageSend{Content: "Usage: !setdad @user"}, nil
	}

	target := m.Mentions[0]
	currentMonth, nextMonth, err := setDadOverride(ctx, c.repo, m.GuildID, target.ID, time.Now())
	if err != nil {
		return nil, err
	}

	announceToChannel(s, c.channelID, target.ID, currentMonth)

	return &discordgo.MessageSend{
		Content: fmt.Sprintf("🍼 <@%s> is Dad of the Month for **%s** and **%s** (baby override)!", target.ID, currentMonth, nextMonth),
	}, nil
}

// setDadOverride writes the baby-override dad for the current month and the next
// month (both flagged is_override). Session-free so it can be unit tested with a
// fake Repository. Returns the formatted current and next month labels.
func setDadOverride(ctx context.Context, repo Repository, guildID, userID string, now time.Time) (string, string, error) {
	if err := repo.insertDad(ctx, guildID, userID, now.Year(), int(now.Month()), true); err != nil {
		return "", "", fmt.Errorf("error setting dad for current month: %w", err)
	}

	next := now.AddDate(0, 1, 0)
	if err := repo.insertDad(ctx, guildID, userID, next.Year(), int(next.Month()), true); err != nil {
		return "", "", fmt.Errorf("error setting dad for next month: %w", err)
	}

	return now.Format("January 2006"), next.Format("January 2006"), nil
}

// pickDadCommand forces a pick for the current month (!pickdad, admin only).
type pickDadCommand struct {
	logger    *slog.Logger
	repo      Repository
	channelID string
	dadRoleID string
}

func NewPickDadCommand(l *slog.Logger, repo Repository, channelID, dadRoleID string) *pickDadCommand {
	return &pickDadCommand{logger: l, repo: repo, channelID: channelID, dadRoleID: dadRoleID}
}

func (c *pickDadCommand) Name() string { return "pickdad" }

func (c *pickDadCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	return isAdmin(s, m) && m.Content == "!pickdad"
}

func (c *pickDadCommand) Handler(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	guild, err := s.Guild(m.GuildID)
	if err != nil {
		return nil, fmt.Errorf("error fetching guild: %w", err)
	}

	if err := PickAndAnnounceForGuild(ctx, c.logger, s, c.repo, guild, c.channelID, c.dadRoleID); err != nil {
		return nil, err
	}

	return nil, nil
}

// whosDadCommand shows the current dad of the month (!whosdad, anyone).
type whosDadCommand struct {
	logger *slog.Logger
	repo   Repository
}

func NewWhosDadCommand(l *slog.Logger, repo Repository) *whosDadCommand {
	return &whosDadCommand{logger: l, repo: repo}
}

func (c *whosDadCommand) Name() string { return "whosdad" }

func (c *whosDadCommand) CanActivate(_ *discordgo.Session, m discordgo.Message) bool {
	return m.Content == "!whosdad"
}

func (c *whosDadCommand) Handler(ctx context.Context, _ *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	now := time.Now()
	entry, err := c.repo.getDadForMonth(ctx, m.GuildID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, fmt.Errorf("error fetching dad of month: %w", err)
	}

	if entry == nil {
		return &discordgo.MessageSend{Content: "No Dad of the Month has been picked yet this month!"}, nil
	}

	month := now.Format("January 2006")
	return &discordgo.MessageSend{
		Content: fmt.Sprintf("🍼 Dad of the Month for **%s** is <@%s>!", month, entry.UserID),
	}, nil
}

func isAdmin(s *discordgo.Session, m discordgo.Message) bool {
	if m.Member == nil || m.Member.Roles == nil {
		return false
	}

	roles, err := s.GuildRoles(m.GuildID)
	if err != nil || roles == nil {
		return false
	}

	return slices.ContainsFunc(m.Member.Roles, func(rID string) bool {
		for _, role := range roles {
			if role.ID == rID && (role.Permissions&discordgo.PermissionAdministrator) != 0 {
				return true
			}
		}
		return false
	})
}
