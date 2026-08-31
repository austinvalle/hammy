package llm

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

//go:embed tpl/image_gen.tpl
var enhanceImagePromptTemplate string

// take text, send to stable-diffusion prompt generator. Take that response and send to dezgo
const (
	dezgoUrl = "https://api.dezgo.com/"
)

var dezgoModels = []string{"nightmareshaper", "deliberate_2", "deliberate_2"} //I want it to be more likely deliberated for now

var dezgoHTTPClient = &http.Client{}

type ImageRequestPayload struct {
	Steps          int     `json:"steps"`
	Prompt         string  `json:"prompt"`
	Model          string  `json:"model"`
	Format         string  `json:"format"`
	Guidance       float32 `json:"guidance"`
	NegativePrompt string  `json:"negative_prompt"`
}

func (l *LLM) GenerateImage(ctx context.Context, prompt string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	enhance := l.EnhanceImagePrompt.Load()
	// Attempt to use the template for the enhanced image prompt
	tmpl, err := useTemplate(enhanceImagePromptTemplate, struct{ Prompt string }{Prompt: prompt})
	if err != nil {
		l.logger.Error("error generating image prompt template", "error", err)
		enhance = false
	}

	// Set initial prompt as default
	finalPrompt := prompt

	// Generate enriched prompt if enhancement is enabled
	if enhance {
		ePrompt, eErr := l.ollama.generate(ctx, hammy, tmpl)
		if eErr != nil {
			l.logger.Warn("error generating enriched prompt, using original prompt", "original prompt", prompt, "error", eErr)
		} else {
			l.logger.Info("generated enriched prompt", "original prompt", prompt, "enriched prompt", ePrompt)
			finalPrompt = ePrompt
		}
	}

	payload := ImageRequestPayload{
		Steps:          30,
		Prompt:         finalPrompt,
		Model:          dezgoModels[rand.Intn(len(dezgoModels))],
		Format:         "jpg",
		Guidance:       l.Guidance(),
		NegativePrompt: "tiling, poorly drawn hands, poorly drawn feet, poorly drawn face, out of frame, extra limbs, disfigured, deformed, body out of frame, blurry, bad anatomy, blurred, watermark, grainy, signature, cut off, draft",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dezgoUrl+"/text2image", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Add("X-Dezgo-Key", l.dezgoToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := dezgoHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("received non-2xx response status: %d", resp.StatusCode)
	}

	l.logger.Info("image generated",
		"cost", resp.Header.Get("x-dezgo-job-amount-usd"),
		"balance", resp.Header.Get("x-dezgo-balance-total-usd"))

	return io.ReadAll(resp.Body)
}
