package telegram

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"music-bot/internal/search"
	"os"
	"strconv"
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
	query := msg.Text

	tracks, err := b.searcher.Search(query)
	if err != nil {
		text := fmt.Sprintf("Ошибка при поиске: %v", err)
		b.sendTextMessage(chatID, text)
		return
	}
	if len(tracks) == 0 {
		b.sendTextMessage(chatID, "Ничего не найдено :(")
		return
	}

	b.lastSearchResults[chatID] = tracks

	var btns []tgbotapi.InlineKeyboardButton
	for i, t := range tracks {
		textBtn := fmt.Sprintf("%s (%s)", t.Title, t.Artist)
		data := strconv.Itoa(i)
		btn := tgbotapi.NewInlineKeyboardButtonData(textBtn, data)
		btns = append(btns, btn)
	}

	kb := tgbotapi.NewInlineKeyboardMarkup(btns)
	outMsg := tgbotapi.NewMessage(chatID, "Выберите трек:")
	outMsg.ReplyMarkup = kb
	b.api.Send(outMsg)
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
		log.Printf("No tracks for chat %d or invalid idx %d", chatID, idx)
		return
	}

	track := tracks[idx]

	tmpFile := fmt.Sprintf("/tmp/%s.mp3", track.ID)
	err = b.searcher.DownloadAudio(track, tmpFile)
	if err != nil {
		log.Printf("Download error: %v", err)
		b.sendTextMessage(chatID, "Ошибка скачивания :(")
		return
	}
	defer os.Remove(tmpFile)

	audioMsg := tgbotapi.NewAudioUpload(chatID, tgbotapi.FilePath(tmpFile))
	audioMsg.Title = track.Title
	audioMsg.Performer = track.Artist

	if _, err := b.api.Send(audioMsg); err != nil {
		log.Printf("Send audio error: %v", err)
		b.sendTextMessage(chatID, "Не удалось отправить аудио.")
	}

	b.api.Request(tgbotapi.NewCallback(cb.ID, "Отправляем аудио..."))
}

func (b *Bot) sendTextMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	b.api.Send(msg)
}
