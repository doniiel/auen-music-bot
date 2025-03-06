package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"log"
	i18n2 "music-bot/internal/i18"
	"music-bot/internal/search"
	"os"
	"strconv"
	"strings"
)

type Bot struct {
	api               *tgbotapi.BotAPI
	searcher          *search.YTSearcher
	lastSearchResults map[int64][]search.Track
	userLang          map[int64]string
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
		userLang:          make(map[int64]string),
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

	switch query {
	case "/start":
		// Отправляем сообщение для выбора языка
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("English", "lang_en"),
				tgbotapi.NewInlineKeyboardButtonData("Русский", "lang_ru"),
			),
		)
		startMsg := tgbotapi.NewMessage(chatID, "Welcome! Please select your language:")
		startMsg.ReplyMarkup = keyboard
		b.api.Send(startMsg)
		return
	case "/help":
		// Пример: здесь можно брать сообщение из локализации
		b.sendTextMessage(chatID, "Help message goes here...")
		return
	case "/search":
		b.sendTextMessage(chatID, "Please type a song title or artist for search.")
		return
	}

	// Если не команда выбора языка, продолжаем обработку запроса (например, поиск)
	loadingMsg, err := b.api.Send(tgbotapi.NewMessage(chatID, "Searching tracks..."))
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
	tracks, err := b.searcher.Search(query)
	if err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("Search error: %v", err))
		return
	}
	if len(tracks) == 0 {
		b.editTextMessage(chatID, loadingMsg.MessageID, "No tracks found.")
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
	photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath("banner.jpeg"))
	photoMsg.Caption = "🎵 Найдены треки:\nВыберите трек:"
	photoMsg.ReplyMarkup = &kb
	if _, err := b.api.Send(photoMsg); err != nil {
		log.Printf("Ошибка отправки фото: %v", err)
	}

}

func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data

	// Если callback содержит выбор языка
	if data == "lang_en" || data == "lang_ru" {
		b.userLang[chatID] = data // сохраняем выбранный язык, например "lang_en" или "lang_ru"
		// Отправляем сообщение с подтверждением
		b.sendTextMessage(chatID, "Language set successfully!")
		return
	}

	// Далее – обработка callback для выбора трека (оставляем текущую логику)
	idx, err := strconv.Atoi(data)
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
	audioMsg.Thumb = tgbotapi.FilePath("logo.png")
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

func (b *Bot) localizeMessage(chatID int64, messageID string) string {
	lang := "en" // язык по умолчанию
	if l, ok := b.userLang[chatID]; ok {
		if l == "lang_ru" {
			lang = "ru"
		}
	}
	localizer := i18n.NewLocalizer(i18n2.Bundle, lang)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})
	if err != nil {
		return messageID // если произошла ошибка, вернуть ключ
	}
	return msg
}
