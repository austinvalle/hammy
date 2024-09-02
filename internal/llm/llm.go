package llm

import (
	"context"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/chromedp/chromedp"
	"github.com/ollama/ollama/api"
	"log/slog"
	"time"
)

const (
	hammy      = "hammy"
	userRole   = "user"
	systemRole = "system"
	botRole    = "assistant"
)

type LLM struct {
	logger *slog.Logger
	hammy  syncClient
}

type syncClient interface {
	Chat(ctx context.Context, messages []api.Message) (string, error)
	Generate(ctx context.Context, systemMessage string, prompt string) (string, error)
}

func NewLLM(logger *slog.Logger, url string) (*LLM, error) {
	client, err := newSyncClientImpl(hammy, url, logger)

	if err != nil {
		return nil, fmt.Errorf("new client error: %w", err)
	}

	return &LLM{
		logger: logger,
		hammy:  client,
	}, nil
}

func (l *LLM) Analyze(ctx context.Context, url string, message *discordgo.MessageCreate) (string, error) {
	content, err := extractContent(ctx, url)
	if err != nil {
		return "", fmt.Errorf("error parsing html %w", err)
	}
	l.logger.Debug("retrieved website content", "content", content)

	systemMsg := fmt.Sprintf(
		`You are a friendly Discord bot named hammy and you are being asked a question about the following content pulled from a website. You are responding directly to the user who asked the question, use "%s" to mention them in discord.
		Read the content and answer the following user question. If they say "analyze", or are only giving you a url just provide a simple summary of it. You can disregard any images and extra stuff that is not related to the content of the article itself.
		%s`, message.Author.Mention(), content)

	t := time.Now()

	defer func(start time.Time) {
		elapsed := time.Since(start)
		l.logger.Info("llm call completed", "elapsed", elapsed)
	}(t)

	return l.hammy.Generate(ctx, systemMsg, message.Content)
}

func (l *LLM) Chat(ctx context.Context, messages []*discordgo.Message) (string, error) {
	msgs := make([]api.Message, 0, len(messages)+1)

	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		role := userRole

		if msg.Author.Bot {
			role = botRole
		}

		// Append messages in correct order
		msgs = append(msgs, api.Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	return l.hammy.Chat(ctx, msgs)
}

func extractContent(ctx context.Context, url string) (string, error) {
	dpCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	var fallbackResult string
	err := chromedp.Run(dpCtx,
		chromedp.Navigate(url),
		chromedp.Text("body", &fallbackResult),
	)
	if err != nil {
		return "", err
	}

	return fallbackResult, nil
}
