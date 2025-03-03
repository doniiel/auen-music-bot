package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"music-bot/internal/search"
	"os"
	"strconv"
	"strings"
)

type Bot struct {
	api               *tgbotapi.BotAPI
	searcher          *search.YTSearcher
	lastSearchResults map[int64][]search.Track
}

func NewBot(token string, searcher *search.YTSearcher) (*Bot, error) {
	botAPI, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	botAPI.Debug = true

	return &Bot{
		api:               botAPI,
		searcher:          searcher,
		lastSearchResults: make(map[int64][]search.Track),
	}, nil
}

func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)
	log.Printf("Bot authorized on account %s", b.api.Self.UserName)

	for update := range updates {
		if update.Message != nil && update.Message.Text != "" {
			b.handleMessage(update.Message)
		}
		if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
		}
	}

	return nil
}
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	query := strings.TrimSpace(msg.Text)

	// Handle specific commands.
	switch query {
	case "/start":
		b.sendTextMessage(chatID, "Привет! Напиши название песни, которую ищешь, и я помогу найти её.")
		return
	case "/help":
		helpText := "Доступные команды:\n" +
			"/start - Приветствие и инструкция\n" +
			"/help - Список команд\n" +
			"/search - Поиск музыки (напиши название песни или исполнителя)"
		b.sendTextMessage(chatID, helpText)
		return
	case "/search":
		b.sendTextMessage(chatID, "Напиши название песни или исполнителя для поиска.")
		return
	}

	// Otherwise, treat the input as a search query.
	loadingMsg, err := b.api.Send(tgbotapi.NewMessage(chatID, "Ищу треки..."))
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}

	// Search for tracks using yt-dlp.
	tracks, err := b.searcher.Search(query)
	if err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("Ошибка при поиске: %v", err))
		return
	}
	if len(tracks) == 0 {
		b.editTextMessage(chatID, loadingMsg.MessageID, "Ничего не найдено.")
		return
	}
	b.lastSearchResults[chatID] = tracks

	// Build a vertical inline keyboard (each button on its own row).
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, track := range tracks {
		btnText := fmt.Sprintf("%s (%s)", track.Title, track.Artist)
		data := strconv.Itoa(i)
		button := tgbotapi.NewInlineKeyboardButtonData(btnText, data)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{button})
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath("jpeg"))
	photoMsg.Caption = "🎵 Найдены треки:\nВыберите трек:"
	photoMsg.ReplyMarkup = &kb
	if _, err := b.api.Send(photoMsg); err != nil {
		log.Printf("Ошибка отправки фото: %v", err)
	}

}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	idx, err := strconv.Atoi(cb.Data)
	if err != nil {
		log.Printf("Callback parse error: %v", err)
		return
	}
	tracks, ok := b.lastSearchResults[chatID]
	if !ok || idx < 0 || idx >= len(tracks) {
		log.Printf("No track for chat %d at index %d", chatID, idx)
		return
	}
	track := tracks[idx]

	// Inform user that download is in progress
	loadingMsg, err := b.api.Send(tgbotapi.NewMessage(chatID, "Downloading audio..."))
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
	tmpFile := fmt.Sprintf("/tmp/%s.mp3", track.ID)
	err = b.searcher.DownloadAudio(track, tmpFile)
	if err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("Error downloading audio: %v", err))
		return
	}
	defer os.Remove(tmpFile)

	audioMsg := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(tmpFile))
	audioMsg.Title = track.Title
	audioMsg.Performer = track.Artist
	if _, err := b.api.Send(audioMsg); err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("Error sending audio: %v", err))
	} else {
		b.editTextMessage(chatID, loadingMsg.MessageID, "Audio delivered successfully!")
	}
	b.api.Request(tgbotapi.NewCallback(cb.ID, "Processing complete."))
}

// sendTextMessage is a helper function to send a simple text message.
func (b *Bot) sendTextMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

// editTextMessage is a helper function to edit an existing message.
func (b *Bot) editTextMessage(chatID int64, messageID int, text string) {
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if _, err := b.api.Send(editMsg); err != nil {
		log.Printf("Ошибка редактирования сообщения: %v", err)
	}
}
