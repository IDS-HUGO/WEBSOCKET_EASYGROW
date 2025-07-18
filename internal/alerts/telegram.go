package alerts

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
)

// Telegram Bot - GRATIS y MUY CONFIABLE
func SendTelegramAlert(message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		return fmt.Errorf("❌ Credenciales de Telegram no configuradas")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	data := url.Values{}
	data.Set("chat_id", chatID)
	data.Set("text", message)
	data.Set("parse_mode", "HTML")

	resp, err := http.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error enviando Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ Telegram falló: %s", resp.Status)
	}

	return nil
}
