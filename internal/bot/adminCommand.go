package bot

import (
	"context"
	"fmt"
	"github.com/austinvalle/hammy/internal/llm"
	"github.com/bwmarrin/discordgo"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
)

var tempCommand = regexp.MustCompile(`setTemperature (?P<temp>\d\.?\d?)`)
var getSettingsCommand = regexp.MustCompile(`.*getSettings*`)
var resetContextCommand = regexp.MustCompile(`resetContext`)
var imageEnhancementCommand = regexp.MustCompile(`setImageEnhancement (?P<enable>on|off)`)
var setGuidanceCommand = regexp.MustCompile(`setGuidance (?P<guidance>\d\.?\d?)`)

const resetMessage = "context reset to here!"

type adminCommand struct {
	logger *slog.Logger
	llm    *llm.LLM
}

func newAdminCommand(logger *slog.Logger, llm *llm.LLM) *adminCommand {
	return &adminCommand{
		logger: logger,
		llm:    llm,
	}
}

func (a *adminCommand) Name() string {
	return "admin"
}

func (a *adminCommand) CanActivate(s *discordgo.Session, m discordgo.Message) bool {
	if resetContextCommand.MatchString(m.Content) || imageEnhancementCommand.MatchString(m.Content) || setGuidanceCommand.MatchString(m.Content) {
		return true //anyone should be allowed to do this
	}

	return isAuthorAdmin(s, m) && (tempCommand.MatchString(m.Content) || getSettingsCommand.MatchString(m.Content))
}

func (a *adminCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	switch {
	case tempCommand.MatchString(m.Content):
		temp, err := extractTemp(m.Content)
		if err != nil {
			return nil, err
		}

		a.llm.SetTemperature(temp)

		return &discordgo.MessageSend{
			Content: fmt.Sprintf("temperature set to %.2f", temp),
		}, nil
	case getSettingsCommand.MatchString(m.Content):
		return &discordgo.MessageSend{
			Content: fmt.Sprintf("current settings\n```json\n%+v\n```", a.llm.GetSettings()),
		}, nil
	case resetContextCommand.MatchString(m.Content):
		//this is a flag we will search for in chat to truncate messages
		return &discordgo.MessageSend{
			Content: resetMessage,
		}, nil
	case imageEnhancementCommand.MatchString(m.Content):
		matches := imageEnhancementCommand.FindStringSubmatch(m.Content)
		if len(matches) != 2 {
			return nil, fmt.Errorf("could not extract enable bool")
		}

		enhance := matches[1] == "on"

		a.llm.EnhanceImagePrompt.Store(enhance)
		return &discordgo.MessageSend{
			Content: fmt.Sprintf("image prompt enhacement set to `%s`", matches[1]),
		}, nil
	case setGuidanceCommand.MatchString(m.Content):
		matches := setGuidanceCommand.FindStringSubmatch(m.Content)
		if len(matches) != 2 {
			return nil, fmt.Errorf("could not extract guidance value")
		}

		guidance, err := strconv.ParseFloat(matches[1], 32)
		if err != nil {
			return nil, fmt.Errorf("could not extract guidance value")
		}
		if guidance > 20 || guidance < -20 {
			return &discordgo.MessageSend{
				Content: "guidance must be between -20 and 20",
			}, nil
		}
		a.llm.SetGuidance(float32(guidance))

		return &discordgo.MessageSend{
			Content: fmt.Sprintf("guidance set to %.2f", guidance),
		}, nil
	}

	return nil, fmt.Errorf("could not match admin command")
}

func extractTemp(message string) (float32, error) {
	idx := tempCommand.SubexpIndex("temp")
	matches := tempCommand.FindStringSubmatch(message)
	if idx != -1 && len(matches) > idx {
		temp, err := strconv.ParseFloat(matches[idx], 32)
		if err != nil {
			return 0, err
		}
		return float32(temp), nil
	}
	return 0, fmt.Errorf("could not match admin command")
}

func isAuthorAdmin(s *discordgo.Session, m discordgo.Message) bool {
	if m.Member == nil || m.Member.Roles == nil {
		return false
	}

	roles, err := s.GuildRoles(m.GuildID)
	if err != nil || roles == nil {
		return false
	}

	return slices.ContainsFunc(m.Member.Roles, func(rId string) bool {
		for _, role := range roles {
			if role.ID == rId && (role.Permissions&discordgo.PermissionAdministrator) != 0 {
				return true
			}
		}
		return false
	})
}
