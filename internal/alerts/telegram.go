package alerts

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Enviar alerta por Telegram - GRATIS y MUY CONFIABLE
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

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error enviando Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ Telegram falló: %s", resp.Status)
	}

	return nil
}

// Enviar alerta personalizada a usuario específico por Telegram
func SendTelegramAlertToUser(phone, message string) error {
	// Por ahora usamos el mismo chat ID configurado
	// En el futuro puedes mapear números de teléfono a chat IDs específicos
	return SendTelegramAlert(fmt.Sprintf("📱 Alerta para %s:\n%s", phone, message))
}
