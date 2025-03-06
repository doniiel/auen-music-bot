package main

import (
	"log"
	i18n "music-bot/internal/i18"

	"music-bot/internal/config"
	"music-bot/internal/logger"
	"music-bot/internal/search"
	"music-bot/internal/telegram"
)

func main() {
	logger.InitLogger()
	i18n.InitI18n()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ytSearcher := search.NewYTSearcher(cfg.YtDlpPath, cfg.SearchLimit)

	bot, err := telegram.NewBot(cfg.TelegramBotToken, ytSearcher)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}

	log.Println("Bot is starting...")
	if err := bot.Start(); err != nil {
		log.Fatalf("bot stopped with error: %v", err)
	}
}
