package bot

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
)

type botContext struct {
	session *discordgo.Session
	logger  *slog.Logger
}

type InteractionHandler func(botContext, *discordgo.InteractionCreate) error

func RunBot(l *slog.Logger, session *discordgo.Session) error {
	err := session.Open()
	if err != nil {
		return fmt.Errorf("unable to connect bot to discord: %w", err)
	}

	logger := createBotLogger(l, session)
	logger.Info("bot successfully connected...")

	registerBotCommands(logger, session)

	defer session.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	logger.Info("bot shutting down...")

	return nil
}

func registerBotCommands(l *slog.Logger, s *discordgo.Session) {
	safeRegister(l, s, ping, pingName, pingDescription)

	go listGlobalCommands(l, s)
}

func safeRegister(l *slog.Logger, s *discordgo.Session, handler InteractionHandler, interactionName string, interactionDesc string) {
	_, err := s.ApplicationCommandCreate(s.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        interactionName,
		Description: interactionDesc,
	})
	if err != nil {
		l.Warn("unable to register command", "err", err, "command.name", interactionName)
	}

	s.AddHandler(createInteractionHandler(l, handler, interactionName))
}

func createInteractionHandler(l *slog.Logger, handler InteractionHandler, slashName string) func(s *discordgo.Session, event *discordgo.InteractionCreate) {
	return func(s *discordgo.Session, event *discordgo.InteractionCreate) {
		if slashName == event.ApplicationCommandData().Name {
			logger := createInteractionLogger(l, handler, event)

			logger.Debug("invoking handler")
			err := handler(botContext{session: s, logger: logger}, event)
			if err != nil {
				logger.Error("error invoking handler", "err", err)
			}
		}
	}
}

func listGlobalCommands(l *slog.Logger, s *discordgo.Session) {
	cmds, err := s.ApplicationCommands(s.State.User.ID, "")
	if err != nil {
		l.Warn("couldn't list global commands", "err", err)
		return
	}

	for _, cmd := range cmds {
		l.Info("command registered globally", "command.name", cmd.Name)
	}
}
