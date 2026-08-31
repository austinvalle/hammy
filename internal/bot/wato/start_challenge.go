package wato

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// startChallengeCommand contains a text command that responds to a user mentioning another user
// along with the phrase "what are the odds ". This will prompt hammy to create a new challenge
// in the database and send an embed message to the same channel where the initial message was
// receive from.
//
// The next command for a challenge will come from either the acceptChallengeCommand
// or declineChallengeCommand.
type startChallengeCommand struct {
	logger *slog.Logger
	dbPool *pgxpool.Pool
}

func NewWatoStartChallengeCommand(logger *slog.Logger, dbPool *pgxpool.Pool) *startChallengeCommand {
	return &startChallengeCommand{
		logger: logger,
		dbPool: dbPool,
	}
}

func (c *startChallengeCommand) Name() string {
	return "wato-start-challenge"
}

func (c *startChallengeCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	// Verify it's in a text channel
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		c.logger.Error("error getting channel ID for startChallengeCommand", "err", err)
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

	return strings.Contains(strings.ToLower(m.Content), `what are the odds `)
}

func (c *startChallengeCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	challenger := m.Author
	challenged := m.Mentions[0] // guaranteed by the CanActivate above

	// The message matches the intent, but let's provide some feedback if the challenge itself isn't possible
	vEmbed, err := c.runValidation(challenger, challenged)
	if err != nil {
		return nil, err
	} else if vEmbed != nil {
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, vEmbed)
		return nil, err
	}

	challenge := challenge{
		ChallengerID: challenger.ID,
		ChallengedID: challenged.ID,
		ChannelID:    m.ChannelID,
		Description:  removeMentionsFromMessage(m.Message),
		Status:       pendingAccept,
	}

	err = createNewChallenge(c.dbPool, challenge)
	if err != nil {
		return nil, err
	}

	activeChallenge, err := getActiveChallenge(c.dbPool, challenger.ID)
	if err != nil {
		return nil, err
	}

	statusEmbed := watoStatusEmbed(challenger, challenged, activeChallenge.Status, activeChallenge.Description)
	statusMessage, err := s.ChannelMessageSendEmbed(m.ChannelID, statusEmbed)
	if err != nil {
		return nil, err
	}

	return nil, setStatusMessageID(c.dbPool, *activeChallenge, statusMessage.ID)
}

// runValidation will return a message embed to send if validation fails
func (c *startChallengeCommand) runValidation(challenger, challenged *discordgo.User) (*discordgo.MessageEmbed, error) {
	if challenger.Bot || challenged.Bot {
		return validationErrEmbed("Bots can't play the odds game."), nil
	}

	if challenger.ID == challenged.ID {
		return validationErrEmbed("You can't challenge yourself."), nil
	}

	if challengerActiveChallenge, err := getActiveChallenge(c.dbPool, challenger.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else if challengerActiveChallenge != nil {
		return validationErrEmbed("You're already in a challenge! Finish that one first."), nil
	}

	if challengedActiveChallenge, err := getActiveChallenge(c.dbPool, challenged.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	} else if challengedActiveChallenge != nil {
		return validationErrEmbed(fmt.Sprintf("<@%s> is already in a challenge! They need to finish that one first.", challenged.ID)), nil
	}

	return nil, nil
}

func validationErrEmbed(msg string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       "WATO Error",
		Description: msg,
		Color:       0xff2821,
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "'Need help? Just type !wato",
			IconURL: "https://i.imgur.com/DbxSPZy.png",
		},
	}
}

func watoStatusEmbed(challenger, challenged *discordgo.User, status challengeStatus, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       fmt.Sprintf("__%s__ has challenged __%s__ to a game of odds!", challenger.Username, challenged.Username),
		Description: fmt.Sprintf("```%s```", description),
		Color:       statusColor(status),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "**Status**",
				Value: statusText(status, challenged.ID),
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text:    "'Need help? Just type !wato",
			IconURL: "https://i.imgur.com/DbxSPZy.png",
		},
	}
}

func statusColor(status challengeStatus) int {
	switch status {
	case declined:
		return 0xff2821
	case pendingAccept:
		return 0xfff821
	default:
		return 0x4dff21
	}
}

func statusText(status challengeStatus, challengedUserID string) string {
	switch status {
	case declined:
		return fmt.Sprintf("<@%s> declined :frowning:", challengedUserID)
	case pendingAccept:
		return fmt.Sprintf("Waiting on response from <@%s>", challengedUserID)
	case pendingBets:
		return "Game on!"
	default:
		return "Game is complete!"
	}
}

func removeMentionsFromMessage(m *discordgo.Message) string {
	content := m.Content

	for _, user := range m.Mentions {
		content = strings.NewReplacer(
			"<@"+user.ID+">", "",
			"<@!"+user.ID+">", "",
		).Replace(content)
	}

	return strings.TrimSpace(content)
}
