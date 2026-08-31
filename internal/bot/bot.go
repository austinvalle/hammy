package bot

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/austinvalle/hammy/internal/bot/dad"
	"github.com/austinvalle/hammy/internal/bot/wato"
	"github.com/austinvalle/hammy/internal/command"
	"github.com/austinvalle/hammy/internal/config"
	"github.com/austinvalle/hammy/internal/llm"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RunBot(l *slog.Logger, session *discordgo.Session, cfg config.Config) error {
	ctx := context.Background()

	var model *llm.LLM
	if !cfg.DisableLLM {
		//create llm first, this is initializing the models so it can take some time to come online
		llmModel, llmErr := llm.NewLLM(l, cfg)
		if llmErr != nil {
			return fmt.Errorf("unable to create llm: %w", llmErr)
		}

		model = llmModel
	}

	l.Info("opening session and starting bot")
	err := session.Open()
	if err != nil {
		return fmt.Errorf("unable to connect bot to discord: %w", err)
	}

	logger := createBotLogger(l, session)
	logger.Info("bot successfully connected")

	logger.Info("connecting to database", "host", cfg.DBHost, "port", cfg.DBPort)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	dbpool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return fmt.Errorf("error creating database: %w", err)
	}

	defer dbpool.Close()

	if pErr := dbpool.Ping(context.Background()); pErr != nil {
		return fmt.Errorf("error pinging database: %w", pErr)
	}

	dadRepo := dad.NewRepository(dbpool)
	registerBotCommands(logger, session, model, cfg, dbpool, dadRepo)

	if cfg.DadAnnouncementChannelID != "" && cfg.DadRoleID != "" {
		dad.StartScheduler(ctx, logger, session, dadRepo, cfg.DadAnnouncementChannelID, cfg.DadRoleID)
	}

	_ = session.UpdateStatusComplex(discordgo.UpdateStatusData{
		AFK: false,
	})

	defer session.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	logger.Info("bot shutting down")

	return nil
}

func registerBotCommands(l *slog.Logger, s *discordgo.Session, model *llm.LLM, cfg config.Config, dbPool *pgxpool.Pool, dadRepo dad.Repository) {
	ping := newPingCommand()

	command.RegisterGuildCommand(l, s, ping)
	command.RegisterInteractionCreate(l, s, ping)

	help := newHelpCommand()

	textCommands := []command.TextCommand{help}

	// LLM commands
	if !cfg.DisableLLM {
		adminCommands := newAdminCommand(l, model)
		analyze := newSummarizeCommand(l, model)
		chat := newChatCommand(l, model)
		art := newImageCommand(l, model)

		//order matters they are checked in order
		textCommands = append(textCommands, []command.TextCommand{
			adminCommands,
			art,
			analyze,
			chat,
		}...)
	}

	// Dad of the Month commands
	if cfg.DadAnnouncementChannelID != "" && cfg.DadRoleID != "" {
		textCommands = append(textCommands, []command.TextCommand{
			dad.NewAddDadCommand(l, cfg.DadRoleID),
			dad.NewSetDadCommand(l, dadRepo, cfg.DadAnnouncementChannelID, cfg.DadRoleID),
			dad.NewPickDadCommand(l, dadRepo, cfg.DadAnnouncementChannelID, cfg.DadRoleID),
			dad.NewWhosDadCommand(l, dadRepo),
		}...)
	}

	// WATO commands
	textCommands = append(textCommands, []command.TextCommand{
		wato.NewWatoHelpCommand(l, dbPool),
		wato.NewWatoStartChallengeCommand(l, dbPool),
		wato.NewWatoAcceptChallengeCommand(l, dbPool),
		wato.NewWatoDeclineChallengeCommand(l, dbPool),
		wato.NewWatoPlaceBetCommand(l, dbPool),
	}...)

	command.RegisterTextCommands(l, s, textCommands)
}

func createBotLogger(logger *slog.Logger, session *discordgo.Session) *slog.Logger {
	if session.State != nil && session.State.Ready.User != nil {
		logger = logger.With("bot_username", session.State.Ready.User.Username)
	}

	return logger
}
