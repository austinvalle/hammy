package bot

import (
	"context"
	"github.com/bwmarrin/discordgo"
	"strings"
)

type helpCommand struct {
}

func newHelpCommand() *helpCommand {
	return &helpCommand{}
}

func (c *helpCommand) Name() string { return "help" }

func (c *helpCommand) CanActivate(_ *discordgo.Session, m discordgo.Message) bool {
	return strings.HasPrefix(m.Content, "!help")
}
func (c *helpCommand) Handler(_ context.Context, s *discordgo.Session, m *discordgo.MessageCreate) (*discordgo.MessageSend, error) {
	helpMessage := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       "How can I help?",
		Description: "Here is what I can do",
		Color:       0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "URL Analyzer",
				Value: "Type `analyze {url}` or just ask me about a question like `{your question} {url}` and I'll give you insights about the link.",
			},
			{
				Name:  "Create Art",
				Value: "Use `!art {your idea}` to generate unique artwork!",
			},
			{
				Name: "Tune image generation",
				Value: "Use `setImageEnhancement {on or off}` to change Hammy's ability to enhance your art prompt! " +
					"Additionally you can use `setGuidance 3.4` or similar to change the guidance used for image generation. " +
					"6-10 is going to be less artifacts but more strict.",
			},
			{
				Name:  "WATO (What Are The Odds)",
				Value: "Challenge someone with `what are the odds {challenge}` for a fun game!",
			},
			{
				Name: "Dad of the Month",
				Value: "I crown a Dad of the Month on the 1st of each month and keep it fair over time.\n" +
					"• `!whosdad` — see the current Dad of the Month\n" +
					"• `!adddad @user` — add someone to the dad pool\n" +
					"• `!setdad @user` — *(admin)* override the pick for this month and next (e.g. new baby)\n" +
					"• `!pickdad` — *(admin)* force a pick for the current month",
			},
			{
				Name:  "Reset Context",
				Value: "type `resetContext` to help me get back on track if I lost my way! This will clear my memory of previous commands / conversation",
			},
		},
	}

	_, err := s.ChannelMessageSendEmbed(m.ChannelID, helpMessage)

	return nil, err
}
