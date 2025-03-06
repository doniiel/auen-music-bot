package i18n

import (
	"embed"
	"encoding/json"
	"log"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var Bundle *i18n.Bundle

func InitI18n() {
	Bundle = i18n.NewBundle(language.English)
	Bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Загружаем файлы перевода
	_, err := Bundle.LoadMessageFileFS(localeFS, "locales/en.json")
	if err != nil {
		log.Fatalf("failed to load en translations: %v", err)
	}
	_, err = Bundle.LoadMessageFileFS(localeFS, "locales/ru.json")
	if err != nil {
		log.Fatalf("failed to load ru translations: %v", err)
	}
	_, err = Bundle.LoadMessageFileFS(localeFS, "locales/kaz.json")
	if err != nil {
		log.Fatalf("failed to load kaz translations: %v", err)
	}
}
