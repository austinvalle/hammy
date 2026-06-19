package dad

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

// StartScheduler runs the monthly dad-of-month picker. On startup it checks
// whether the current month has a dad set and catches up if not. Then it sleeps
// until the 1st of each month at 10am EST and picks automatically.
func StartScheduler(ctx context.Context, l *slog.Logger, s *discordgo.Session, repo Repository, channelID, dadRoleID string) {
	go func() {
		now := time.Now()
		if err := runPickIfNeeded(ctx, l, s, repo, channelID, dadRoleID, now); err != nil {
			l.Error("dad-of-month catch-up pick failed", "err", err)
		}

		for {
			next := nextPickTime(time.Now())
			l.Info("dad-of-month scheduler sleeping", "next_pick", next)

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
			}

			now := time.Now()
			if err := runPickIfNeeded(ctx, l, s, repo, channelID, dadRoleID, now); err != nil {
				l.Error("dad-of-month scheduled pick failed", "err", err)
			}
		}
	}()
}

func runPickIfNeeded(ctx context.Context, l *slog.Logger, s *discordgo.Session, repo Repository, channelID, dadRoleID string, now time.Time) error {
	guilds := s.State.Guilds
	for _, guild := range guilds {
		existing, err := repo.getDadForMonth(ctx, guild.ID, now.Year(), int(now.Month()))
		if err != nil {
			l.Error("error checking existing dad of month", "guild", guild.ID, "err", err)
			continue
		}

		hasDad := existing != nil
		if !shouldCatchUp(now, hasDad) {
			continue
		}

		if err := pickAndAnnounce(ctx, l, s, repo, guild, channelID, dadRoleID, now, false); err != nil {
			l.Error("error picking dad of month", "guild", guild.ID, "err", err)
		}
	}
	return nil
}

// PickAndAnnounceForGuild is called by the !pickdad command to force a pick.
func PickAndAnnounceForGuild(ctx context.Context, l *slog.Logger, s *discordgo.Session, repo Repository, guild *discordgo.Guild, channelID, dadRoleID string) error {
	return pickAndAnnounce(ctx, l, s, repo, guild, channelID, dadRoleID, time.Now(), false)
}

func pickAndAnnounce(ctx context.Context, l *slog.Logger, s *discordgo.Session, repo Repository, guild *discordgo.Guild, channelID, dadRoleID string, now time.Time, isOverride bool) error {
	candidates, err := getMembersWithRole(s, guild, dadRoleID)
	if err != nil {
		return fmt.Errorf("error fetching dad role members: %w", err)
	}

	if len(candidates) == 0 {
		l.Warn("no candidates with dad role found", "guild", guild.ID)
		return nil
	}

	history, err := repo.getHistory(ctx, guild.ID)
	if err != nil {
		return fmt.Errorf("error fetching dad history: %w", err)
	}

	chosen := selectDad(candidates, history, now)

	if err := repo.insertDad(ctx, guild.ID, chosen, now.Year(), int(now.Month()), isOverride); err != nil {
		return fmt.Errorf("error inserting dad of month: %w", err)
	}

	month := now.Format("January 2006")
	announceToChannel(s, channelID, chosen, month)
	l.Info("dad of month picked", "guild", guild.ID, "user", chosen, "month", month)
	return nil
}

func announceToChannel(s *discordgo.Session, channelID, userID, month string) {
	msg := fmt.Sprintf("🍼 **Dad of the Month for %s: <@%s>!** Congratulations!", month, userID)
	_, _ = s.ChannelMessageSend(channelID, msg)
}

func getMembersWithRole(s *discordgo.Session, guild *discordgo.Guild, roleID string) ([]string, error) {
	members, err := s.GuildMembers(guild.ID, "", 1000)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, m := range members {
		for _, r := range m.Roles {
			if r == roleID {
				result = append(result, m.User.ID)
				break
			}
		}
	}
	return result, nil
}
