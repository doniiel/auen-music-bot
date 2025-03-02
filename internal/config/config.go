package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	TelegramBotToken string
	YtDlpPath        string
	SearchLimit      int
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}
	cfg.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}

	cfg.YtDlpPath = os.Getenv("TELEGRAM_DLP_PATH")
	if cfg.YtDlpPath == "" {
		cfg.YtDlpPath = "yt-dlp"
	}

	limitStr := os.Getenv("SEARCH_LIMIT")
	if limitStr == "" {
		cfg.YtDlpPath = "10"
	} else {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("SEARCH_LIMIT invalid: %w", err)
		}
		cfg.SearchLimit = limit
	}
	return cfg, nil
}
