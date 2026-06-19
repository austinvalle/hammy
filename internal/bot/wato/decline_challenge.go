package wato

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// declineChallengeCommand contains a text command that rejects the active challenge of a user.
type declineChallengeCommand struct {
	logger *slog.Logger
	dbPool *pgxpool.Pool
}

func NewWatoDeclineChallengeCommand(logger *slog.Logger, dbPool *pgxpool.Pool) *declineChallengeCommand {
	return &declineChallengeCommand{
		logger: logger,
		dbPool: dbPool,
	}
}

func (c *declineChallengeCommand) Name() string {
	return "wato-decline-challenge"
}

func (c *declineChallengeCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	// Verify it's in a text channel
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		c.logger.Error("error getting channel ID for declineChallengeCommand", "err", err)
		return false
	}

	if channel.Type != discordgo.ChannelTypeGuildText {
		return false
	}

	// Verify there is only one unique mention
	uniqueMention := false
	for _, mention := range m.Mentions {
		if mention.ID != m.Author.ID {
			uniqueMention = true
			break
		}
	}

	if !uniqueMention || len(m.Mentions) > 1 {
		return false
	}

	validDecline := strings.Contains(strings.ToLower(m.Content), `decline`)

	if !validDecline {
		return false
	}

	_, err = getActiveChallenge(c.dbPool, m.Author.ID)
	if err != nil {
		// Either there is no active challenge, or we don't know if they have an active challenge
		// because the database is having issues. Just log if needed and return false.
		if !errors.Is(err, pgx.ErrNoRows) {
			c.logger.Error("error getting active challenge from database", "err", err)
		}
		return false
	}

	return true
}

func (c *declineChallengeCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	activeChallenge, err := getActiveChallenge(c.dbPool, m.Author.ID)
	if err != nil {
		// No active challenge, return with no error
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if activeChallenge.Status != pendingAccept || activeChallenge.ChallengedID != m.Author.ID {
		return nil, nil
	}

	err = declineChallenge(c.dbPool, *activeChallenge)
	if err != nil {
		return nil, err
	}

	challenger, err := s.GuildMember(m.GuildID, activeChallenge.ChallengerID)
	if err != nil {
		return nil, err
	}

	updatedStatusEmbed := watoStatusEmbed(challenger.User, m.Author, declined, activeChallenge.Description)
	_, err = s.ChannelMessageEditEmbed(activeChallenge.ChannelID, *activeChallenge.StatusMessageID, updatedStatusEmbed)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
