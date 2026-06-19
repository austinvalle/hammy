package wato

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var validNumberRegex = regexp.MustCompile(`^[-]?\d+(,\d+)*$`)

// acceptChallengeCommand contains a text command that looks for a mention and a number,
// checks for any challenges that the user may be responding to. If an active challenge is
// found, the bet/status message will be updated and will send both players a DM requesting
// their bet.
//
// The next command for a challenge will be the betCommand
type acceptChallengeCommand struct {
	logger *slog.Logger
	dbPool *pgxpool.Pool
}

func NewWatoAcceptChallengeCommand(logger *slog.Logger, dbPool *pgxpool.Pool) *acceptChallengeCommand {
	return &acceptChallengeCommand{
		logger: logger,
		dbPool: dbPool,
	}
}

func (c *acceptChallengeCommand) Name() string {
	return "wato-accept-challenge"
}

func (c *acceptChallengeCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	// Verify it's in a text channel
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		c.logger.Error("error getting channel ID for acceptChallengeCommand", "err", err)
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

	validAccept := validNumberRegex.Match([]byte(removeMentionsFromMessage(&m)))

	if !validAccept {
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

func (c *acceptChallengeCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
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

	numbersFound := validNumberRegex.FindAllString(removeMentionsFromMessage(m.Message), -1)
	if len(numbersFound) < 1 {
		return nil, nil
	}

	betLimit, err := strconv.Atoi(strings.ReplaceAll(numbersFound[0], ",", ""))
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return nil, err
	}

	if errors.Is(err, strconv.ErrRange) || betLimit <= 1 {
		vEmbed := validationErrEmbed(fmt.Sprintf("<@%s> Your bet needs to be between 1 and 9,223,372,036,854,775,808", m.Author.ID))
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, vEmbed)
		return nil, err
	}

	err = setBetLimit(c.dbPool, *activeChallenge, betLimit)
	if err != nil {
		return nil, err
	}

	challengedDirectChannel, err := s.UserChannelCreate(m.Author.ID)
	if err != nil {
		return nil, err
	}

	challenger, err := s.GuildMember(m.GuildID, activeChallenge.ChallengerID)
	if err != nil {
		return nil, err
	}

	challengerDirectChannel, err := s.UserChannelCreate(challenger.User.ID)
	if err != nil {
		return nil, err
	}

	_, err = s.ChannelMessageSendEmbed(challengerDirectChannel.ID, createDMEmbed(m.Author.Username, betLimit, *activeChallenge))
	if err != nil {
		return nil, err
	}

	_, err = s.ChannelMessageSendEmbed(challengedDirectChannel.ID, createDMEmbed(challenger.User.Username, betLimit, *activeChallenge))
	if err != nil {
		return nil, err
	}

	updatedStatusEmbed := watoStatusEmbed(challenger.User, m.Author, pendingBets, activeChallenge.Description)
	_, err = s.ChannelMessageEditEmbed(activeChallenge.ChannelID, *activeChallenge.StatusMessageID, updatedStatusEmbed)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func createDMEmbed(username string, betLimit int, c challenge) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       fmt.Sprintf("__%s__ challenged you!", username),
		Description: fmt.Sprintf("```%s```", c.Description),
		Color:       0xffffff,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "**Status**",
				Value:  fmt.Sprintf("Respond here with a number between 1 and %d", betLimit),
				Inline: true,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "'Need help? Just type !wato",
			IconURL: "https://i.imgur.com/DbxSPZy.png",
		},
	}
}
