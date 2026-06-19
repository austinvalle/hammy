package llm

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"math"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/austinvalle/hammy/internal/config"
	"github.com/bwmarrin/discordgo"
	"github.com/chromedp/chromedp"
	"log/slog"
	"slices"
	"strings"
)

//go:embed tpl/analyze.tpl
var analyzeTemplate string

const (
	hammy = "hammy"
)

// Settings are returned in GetSettings call for checking various settings
type Settings struct {
	Temperature        float32
	EnhanceImagePrompt bool
	Guidance           float32
}

type LLM struct {
	logger             *slog.Logger
	ollama             ollamaClient
	temperature        atomic.Uint32
	dezgoToken         string
	EnhanceImagePrompt atomic.Bool
	guidance           atomic.Uint32
}

func (l *LLM) Temperature() float32 {
	return math.Float32frombits(l.temperature.Load())
}

func (l *LLM) SetTemperature(t float32) {
	l.temperature.Store(math.Float32bits(t))
}

func (l *LLM) Guidance() float32 {
	return math.Float32frombits(l.guidance.Load())
}

func (l *LLM) SetGuidance(g float32) {
	l.guidance.Store(math.Float32bits(g))
}

type ollamaClient interface {
	chat(ctx context.Context, modelName string, message string, history []string, opts ...Options) (string, error)
	generate(ctx context.Context, modelName string, prompt string, opts ...Options) (string, error)
}

func NewLLM(logger *slog.Logger, cfg config.Config) (*LLM, error) {
	client, err := newSyncClientImpl(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("new c error: %w", err)
	}

	logger.Info("configuring models")
	if cErr := client.configure(context.Background()); cErr != nil {
		return nil, fmt.Errorf("configure error: %w", cErr)
	}

	logger.Info("getting temperature")
	temp, err := client.getTemperature(context.Background(), hammy)
	if err != nil {
		return nil, fmt.Errorf("get temperature error: %w", err)
	}

	llm := &LLM{
		logger:     logger,
		ollama:     client,
		dezgoToken: cfg.DezgoToken,
	}

	llm.SetTemperature(temp)
	llm.SetGuidance(3.4)
	llm.EnhanceImagePrompt.Store(cfg.EnhanceImagePrompt)
	return llm, nil
}

func (l *LLM) Analyze(ctx context.Context, url string, message *discordgo.MessageCreate) (string, error) {
	content, err := extractContent(ctx, url)
	if err != nil {
		return "", fmt.Errorf("error parsing html %w", err)
	}
	l.logger.Debug("retrieved website content", "content", content)

	data := map[string]string{
		"mention": message.Author.Mention(),
		"content": content,
		"prompt":  message.Content,
	}

	prompt, err := useTemplate(analyzeTemplate, data)
	if err != nil {
		return "", fmt.Errorf("error building analyze prompt: %w", err)
	}

	t := time.Now()

	defer func(start time.Time) {
		elapsed := time.Since(start)
		l.logger.Info("llm call completed", "elapsed", elapsed)
	}(t)

	return l.ollama.generate(ctx, hammy, prompt, WithTemperature(l.Temperature()))
}

func (l *LLM) Chat(ctx context.Context, m *discordgo.Message, history []*discordgo.Message) (string, error) {
	slices.Reverse(history)

	msgs := make([]string, 0)
	for _, message := range history {
		msgs = append(msgs, formatMessage(message))
	}

	latest := formatMessage(m)
	return l.ollama.chat(ctx, hammy, latest, msgs, WithTemperature(l.Temperature()))
}

func (l *LLM) GetSettings() Settings {
	return Settings{
		Temperature:        l.Temperature(),
		EnhanceImagePrompt: l.EnhanceImagePrompt.Load(),
		Guidance:           l.Guidance(),
	}
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

func useTemplate[T any](t string, data T) (string, error) {
	tpl, err := template.New("chat").Parse(t)
	if err != nil {
		return "", fmt.Errorf("error loading template: %w", err)
	}
	var promptBuffer bytes.Buffer

	if err = tpl.Execute(&promptBuffer, data); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return promptBuffer.String(), nil
}

func formatMessage(m *discordgo.Message) string {
	author := m.Author.GlobalName
	if m.Author.Bot {
		author = "AI"
	}
	content := strings.ReplaceAll(m.Content, "\n", " ")
	return fmt.Sprintf("%s:%s", author, content)
}
