package bot

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/austinvalle/hammy/internal/command"
	"github.com/austinvalle/hammy/internal/llm"
	"github.com/bwmarrin/discordgo"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
)

const (
	urlPattern = ".*(?P<url>https?:\\/\\/(www\\.)?[-a-zA-Z0-9@:%._\\+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\\b([-a-zA-Z0-9()@:%_\\+.~#?&//=]*)).*"
)

var compiledURLRegex = regexp.MustCompile(urlPattern)

type summarizeCommand struct {
	logger *slog.Logger
	llm    *llm.LLM
}

func newSummarizeCommand(logger *slog.Logger, llm *llm.LLM) command.TextCommand {
	return &summarizeCommand{
		logger: logger,
		llm:    llm,
	}
}
func (c *summarizeCommand) Name() string {
	return "analyze"
}

func (c *summarizeCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	if mentioned, err := isHammyMentioned(s, m); err != nil {
		c.logger.Error("error checking mentions", "err", err)
	} else if !mentioned {
		return false
	}

	matches := compiledURLRegex.FindStringSubmatch(m.Content)
	idx := compiledURLRegex.SubexpIndex("url")

	if len(matches) < idx+1 || matches[idx] == "" {
		return false
	}

	return true
}

func (c *summarizeCommand) Handler(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	//todo: dont rawdog viper
	emoji := viper.GetString("ResponseEmoji")
	if err := s.MessageReactionAdd(m.ChannelID, m.ID, emoji); err != nil {
		c.logger.Error("error adding reaction: ", "err", err)
	}

	c.logger.Debug("In summary handler")

	blacklistedSites := []string{"twitter.com", "x.com", "facebook.com", "instagram.com", "reddit.com", "google.com"}

	matches := compiledURLRegex.FindStringSubmatch(m.Content)
	idx := compiledURLRegex.SubexpIndex("url")
	if len(matches) < idx+1 || matches[idx] == "" {
		return nil, fmt.Errorf("URL regex did not match")
	}
	url := matches[idx]
	isBlacklisted := slices.ContainsFunc(blacklistedSites, func(site string) bool {
		return strings.Contains(url, site)
	})

	if isBlacklisted {
		return &discordgo.MessageSend{
			Content: fmt.Sprintf("Hmm, %s it looks like you provided a link that I cannot analyze!", m.Author.Mention()),
		}, nil

	}

	response, err := c.llm.Analyze(ctx, url, m)
	if err != nil {
		c.logger.Error("error in analyzing message", "err", err)
		return nil, err
	}
	if removeError := s.MessageReactionRemove(m.ChannelID, m.ID, emoji, "@me"); removeError != nil {
		c.logger.Error("error removing reaction", "err", removeError)
	}
	return &discordgo.MessageSend{
		Content: response,
	}, nil

}

// Hammy can be referenced inconsistently by role or by mention. We should check both to make sure its correct.
func isHammyMentioned(s *discordgo.Session, m discordgo.Message) (bool, error) {
	var err error
	if mentioned := slices.ContainsFunc(m.MentionRoles, func(roleId string) bool {
		role, rErr := s.State.Role(m.GuildID, roleId)
		if rErr != nil && err != nil {
			err = fmt.Errorf("error getting role: %w", rErr)
			return false
		}
		return role.Name == s.State.User.Username
	}); err != nil {
		return false, err
	} else if mentioned {
		return true, nil
	}

	return slices.ContainsFunc(m.Mentions, func(user *discordgo.User) bool {
		return user.ID == s.State.User.ID
	}), nil

}
