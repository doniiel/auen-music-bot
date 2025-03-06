package telegram

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	i18n2 "music-bot/internal/i18"
	"music-bot/internal/search"
)

type Bot struct {
	api               *tgbotapi.BotAPI
	searcher          *search.YTSearcher
	lastSearchResults map[int64][]search.Track
	userLang          map[int64]string // хранит выбранный язык для каждого чата (например, "lang_ru", "lang_en", "lang_kaz")
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
		// Отправляем сообщение для выбора языка с использованием локализации
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("English", "lang_en"),
				tgbotapi.NewInlineKeyboardButtonData("Русский", "lang_ru"),
				tgbotapi.NewInlineKeyboardButtonData("Қазақша", "lang_kaz"),
			),
		)
		// По умолчанию сообщение /start локализуем как "start" (в ru.json оно должно быть на русском)
		startMsgText := b.localizeMessage(chatID, "start")
		startMsg := tgbotapi.NewMessage(chatID, startMsgText)
		startMsg.ReplyMarkup = keyboard
		b.api.Send(startMsg)
		return
	case "/help":
		helpText := b.localizeMessage(chatID, "help")
		b.sendTextMessage(chatID, helpText)
		return
	case "/search":
		searchPrompt := b.localizeMessage(chatID, "search_prompt")
		b.sendTextMessage(chatID, searchPrompt)
		return
	}

	// Если сообщение не является командой выбора языка, считаем его поисковым запросом.
	loadingMsg, err := b.api.Send(tgbotapi.NewMessage(chatID, b.localizeMessage(chatID, "searching")))
	if err != nil {
		log.Printf("Ошибка отправки сообщения: %v", err)
	}
	tracks, err := b.searcher.Search(query)
	if err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("%s: %v", b.localizeMessage(chatID, "search_error"), err))
		return
	}
	if len(tracks) == 0 {
		b.editTextMessage(chatID, loadingMsg.MessageID, b.localizeMessage(chatID, "no_tracks"))
		return
	}
	b.lastSearchResults[chatID] = tracks

	// Построение вертикальной inline-клавиатуры с результатами.
	var rows [][]tgbotapi.InlineKeyboardButton
	for i, track := range tracks {
		btnText := fmt.Sprintf("%s (%s)", track.Title, track.Artist)
		data := strconv.Itoa(i)
		button := tgbotapi.NewInlineKeyboardButtonData(btnText, data)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{button})
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(rows...)
	// Отправляем фото-баннер с клавиатурой. Убедитесь, что файл "banner.jpeg" скопирован в контейнер.
	photoMsg := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath("asset/banner.jpeg"))
	photoMsg.Caption = b.localizeMessage(chatID, "tracks_found")
	photoMsg.ReplyMarkup = &kb
	if _, err := b.api.Send(photoMsg); err != nil {
		log.Printf("Ошибка отправки фото: %v", err)
	}
}
func (b *Bot) handleCallback(cb *tgbotapi.CallbackQuery) {
	chatID := cb.Message.Chat.ID
	data := cb.Data
	// Обработка выбора языка
	if data == "lang_en" || data == "lang_ru" || data == "lang_kaz" {
		b.userLang[chatID] = data // Сохраняем выбранный язык
		langSetMsg := b.localizeMessage(chatID, "language_set")
		b.sendTextMessage(chatID, langSetMsg)
		// Отправляем инструкцию по поиску музыки
		searchInstr := b.localizeMessage(chatID, "search_instruction")
		b.sendTextMessage(chatID, searchInstr)
		return
	}

	// Обработка выбора трека
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

	loadingMsg, err := b.api.Send(tgbotapi.NewMessage(chatID, b.localizeMessage(chatID, "downloading")))
	if err != nil {
		log.Printf("Error sending message: %v", err)
	}
	tmpFile := fmt.Sprintf("/tmp/%s.mp3", track.ID)
	err = b.searcher.DownloadAudio(track, tmpFile)
	if err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("%s: %v", b.localizeMessage(chatID, "download_error"), err))
		return
	}
	defer os.Remove(tmpFile)

	audioMsg := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(tmpFile))
	audioMsg.Title = track.Title
	audioMsg.Performer = track.Artist
	audioMsg.Thumb = tgbotapi.FilePath("asset/logo.png")
	if _, err := b.api.Send(audioMsg); err != nil {
		b.editTextMessage(chatID, loadingMsg.MessageID, fmt.Sprintf("%s: %v", b.localizeMessage(chatID, "send_audio_error"), err))
	} else {
		b.editTextMessage(chatID, loadingMsg.MessageID, b.localizeMessage(chatID, "audio_delivered"))
	}
	b.api.Request(tgbotapi.NewCallback(cb.ID, b.localizeMessage(chatID, "processing_complete")))
}

// sendTextMessage отправляет простое текстовое сообщение.
func (b *Bot) sendTextMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}

// editTextMessage редактирует существующее сообщение.
func (b *Bot) editTextMessage(chatID int64, messageID int, text string) {
	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if _, err := b.api.Send(editMsg); err != nil {
		log.Printf("Ошибка редактирования сообщения: %v", err)
	}
}

// localizeMessage возвращает локализованное сообщение для данного чата.
func (b *Bot) localizeMessage(chatID int64, messageID string) string {
	// По умолчанию русский язык.
	lang := "ru"
	if l, ok := b.userLang[chatID]; ok {
		if l == "lang_en" {
			lang = "en"
		} else if l == "lang_kaz" {
			lang = "kaz"
		}
	}
	localizer := i18n.NewLocalizer(i18n2.Bundle, lang)
	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})
	if err != nil {
		return messageID // Если произошла ошибка, вернуть ключ.
	}
	return msg
}
