package wato

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// placeBetCommand contains a text command that accepts a DM from a player with an active challenge
// that contains a bet (whole number). This bet will be recorded in the DB, then the command will check if
// the game should be completed.
//
// If both bets have been made, the command will update the DB and send a message embed to the original text
// channel where the challenge was made with the results. The challenger is the winner if they place the same
// bet as the challenged user, otherwise the challenged user wins.
type placeBetCommand struct {
	logger *slog.Logger
	dbPool *pgxpool.Pool
}

func NewWatoPlaceBetCommand(logger *slog.Logger, dbPool *pgxpool.Pool) *placeBetCommand {
	return &placeBetCommand{
		logger: logger,
		dbPool: dbPool,
	}
}

func (c *placeBetCommand) Name() string {
	return "wato-place-bet"
}

func (c *placeBetCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	// Verify it's in a dm channel
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		c.logger.Error("error getting channel ID for placeBetCommand", "err", err)
		return false
	}

	if channel.Type != discordgo.ChannelTypeDM {
		return false
	}

	validBet := validNumberRegex.Match([]byte(m.Content))

	if !validBet {
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

func (c *placeBetCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	activeChallenge, err := getActiveChallenge(c.dbPool, m.Author.ID)
	if err != nil {
		// No active challenge, return with no error
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if activeChallenge.Status != pendingBets || activeChallenge.BetLimit == nil {
		return nil, nil
	}

	numbersFound := validNumberRegex.FindAllString(m.Content, -1)
	if len(numbersFound) < 1 {
		return nil, nil
	}

	bet, err := strconv.Atoi(strings.ReplaceAll(numbersFound[0], ",", ""))
	if err != nil {
		return nil, err
	}

	if bet <= 1 || bet > *activeChallenge.BetLimit {
		p := message.NewPrinter(language.English)
		vEmbed := validationErrEmbed(p.Sprintf("<@%s> Your bet needs to be between 1 and %d", m.Author.ID, *activeChallenge.BetLimit))
		_, err := s.ChannelMessageSendEmbed(m.ChannelID, vEmbed)
		return nil, err
	}

	err = placeUserBet(c.dbPool, *activeChallenge, bet, activeChallenge.ChallengerID == m.Author.ID)
	if err != nil {
		return nil, err
	}

	_, err = s.ChannelMessageSend(m.ChannelID, "Got it!")
	if err != nil {
		return nil, err
	}

	// Check to see if the game is done
	activeChallenge, err = getActiveChallenge(c.dbPool, m.Author.ID)
	if err != nil {
		return nil, err
	}

	// Game isn't done
	if activeChallenge.ChallengerBet == nil || activeChallenge.ChallengedBet == nil || activeChallenge.Status != pendingBets {
		return nil, nil
	}

	return nil, c.completeGame(s, *activeChallenge)
}

func (c *placeBetCommand) completeGame(s *discordgo.Session, activeChallenge challenge) error {
	winnerID := activeChallenge.ChallengedID
	loserID := activeChallenge.ChallengerID

	if *activeChallenge.ChallengerBet == *activeChallenge.ChallengedBet {
		winnerID = activeChallenge.ChallengerID
		loserID = activeChallenge.ChallengedID
	}

	err := updateGameToCompleted(c.dbPool, activeChallenge, winnerID)
	if err != nil {
		return err
	}

	originalChannel, err := s.Channel(activeChallenge.ChannelID)
	if err != nil {
		return err
	}

	winner, err := s.GuildMember(originalChannel.GuildID, winnerID)
	if err != nil {
		return err
	}

	loser, err := s.GuildMember(originalChannel.GuildID, loserID)
	if err != nil {
		return err
	}

	resultsEmbed := watoResultsEmbed(winner.User, loser.User, activeChallenge)
	_, err = s.ChannelMessageSendEmbed(originalChannel.ID, resultsEmbed)
	return err
}

func watoResultsEmbed(winner, loser *discordgo.User, c challenge) *discordgo.MessageEmbed {
	winningBet := *c.ChallengedBet
	losingBet := *c.ChallengerBet

	if winner.ID == c.ChallengerID {
		winningBet = *c.ChallengerBet
		losingBet = *c.ChallengedBet
	}

	p := message.NewPrinter(language.English)

	return &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       fmt.Sprintf("__%s__ has defeated __%s__ in a game of odds!", winner.Username, loser.Username),
		Description: fmt.Sprintf("```%s```", c.Description),
		Color:       0x0099ff,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: winner.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   fmt.Sprintf("%s's bet", winner.Username),
				Value:  p.Sprintf("%d", winningBet),
				Inline: true,
			},
			{
				Name:   fmt.Sprintf("%s's bet", loser.Username),
				Value:  p.Sprintf("%d", losingBet),
				Inline: true,
			},
		},
	}
}
