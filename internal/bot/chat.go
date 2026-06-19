package bot

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"slices"
	"strings"

	"github.com/austinvalle/hammy/internal/llm"
	"github.com/bwmarrin/discordgo"
)

// todo: implement
type chatCommand struct {
	logger *slog.Logger
	llm    *llm.LLM
}

func newChatCommand(logger *slog.Logger, llm *llm.LLM) *chatCommand {
	return &chatCommand{
		logger: logger,
		llm:    llm,
	}
}
func (c *chatCommand) Name() string { return "chat" }
func (c *chatCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	safeChannels := []string{"real-talk", "admin", "reaction-roles", "trip-planning-war-room"}
	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		c.logger.Error("could not get channel details from state", "err", err)
		return false
	}
	if slices.Contains(safeChannels, channel.Name) {
		c.logger.Debug("Message received in safe channel, ignoring chat")
		return false
	}

	if ok, rErr := isHammyMentioned(s, m); rErr != nil {
		c.logger.Error("error checking mentions", "err", rErr)
	} else if ok {
		return true
	}

	//if he was mentioned but not tagged, 10% chance he defends himself
	if strings.Contains(m.Content, "hammy") {
		return rand.Intn(10) == 0
	}

	//otherwise 4% chance of responding
	return rand.Intn(25) == 0
}

func (c *chatCommand) Handler(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	msgs, err := s.ChannelMessages(m.ChannelID, 75, "", "", m.ID)
	if err != nil {
		return nil, err
	}

	//search for the most recent resetContext confirmation and trim history
	idx := slices.IndexFunc(msgs, func(msg *discordgo.Message) bool {
		return msg.Content == resetMessage && msg.Author.Bot
	})

	if idx != -1 {
		msgs = msgs[:idx] //msgs are newest to oldest
	}

	bm, bn := s.State.User.Mention(), s.State.User.Username
	sanitize := func(msg *discordgo.Message) {
		if msg.Content != "" {
			msg.Content = strings.ReplaceAll(msg.Content, bm, bn)
		}
	}

	for _, msg := range msgs {
		sanitize(msg)
	}
	sanitize(m.Message)

	//remove latest from history
	if len(msgs) > 0 {
		msgs = msgs[:len(msgs)-1]
	}
	resp, err := c.llm.Chat(ctx, m.Message, msgs)
	if err != nil {
		return nil, err
	}
	prefix := fmt.Sprintf("%s#%s said:", s.State.User.Username, s.State.User.Discriminator)
	resp, _ = strings.CutPrefix(resp, prefix)
	resp = strings.Trim(resp, "\"")
	return &discordgo.MessageSend{
		Content: resp,
	}, nil
}
