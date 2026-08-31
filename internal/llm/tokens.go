package llm

import (
	"log/slog"
	"sync"

	"github.com/pkoukk/tiktoken-go"
	tiktoken_loader "github.com/pkoukk/tiktoken-go-loader"
)

const encoding = "cl100k_base"

// todo: this is just an estimate and will be wrong, use ollama token endpoints when this is merged https://github.com/ollama/ollama/issues/3582

var (
	tke     *tiktoken.Tiktoken
	tkeOnce sync.Once
	tkeErr  error
)

func initEncoder() {
	tkeOnce.Do(func() {
		tiktoken.SetBpeLoader(tiktoken_loader.NewOfflineLoader())
		tke, tkeErr = tiktoken.GetEncoding(encoding)
	})
}

// counts the tokens in a given string of text using chatgpt tokenizers
func getTokenCount(text string) int {
	initEncoder()
	if tkeErr != nil {
		slog.Default().Warn("tokenizer unavailable, returning 0", "err", tkeErr)
		return 0
	}
	return len(tke.Encode(text, nil, nil))
}
