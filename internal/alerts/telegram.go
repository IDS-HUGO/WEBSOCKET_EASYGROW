package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// Mapeo de números de teléfono a Chat IDs de Telegram
var phoneToTelegramMap = map[string]string{
	"+529727228805": "7521746198", // Tu número específico
	// Agregar más usuarios aquí: "numero": "chat_id"
}

// Enviar alerta por Telegram - GRATIS y MUY CONFIABLE
func SendTelegramAlert(message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		return fmt.Errorf("❌ Credenciales de Telegram no configuradas")
	}

	return sendTelegramMessage(botToken, chatID, message)
}

// Enviar alerta personalizada a usuario específico por Telegram
func SendTelegramAlertToUser(phone, message string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	if botToken == "" {
		return fmt.Errorf("❌ Token de Telegram no configurado")
	}

	// Normalizar el número de teléfono
	normalizedPhone := phone
	if !strings.HasPrefix(phone, "+") {
		normalizedPhone = "+" + phone
	}

	// Buscar el Chat ID específico del usuario
	chatID, exists := phoneToTelegramMap[normalizedPhone]
	if !exists {
		// Si no encontramos el usuario específico, usar el chat ID por defecto
		chatID = os.Getenv("TELEGRAM_CHAT_ID")
		if chatID == "" {
			return fmt.Errorf("❌ No se encontró Chat ID para %s y no hay chat por defecto", normalizedPhone)
		}

		// Incluir el número en el mensaje ya que va al chat grupal
		message = fmt.Sprintf("📱 <b>Alerta para %s:</b>\n%s", normalizedPhone, message)
	}

	return sendTelegramMessage(botToken, chatID, message)
}

// Función auxiliar para enviar el mensaje a Telegram
func sendTelegramMessage(botToken, chatID, message string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	telegramMsg := TelegramMessage{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(telegramMsg)
	if err != nil {
		return fmt.Errorf("❌ Error creando JSON: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("❌ Error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error enviando a Telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ Telegram falló: %s", resp.Status)
	}

	fmt.Printf("✅ Mensaje Telegram enviado exitosamente a Chat ID: %s\n", chatID)
	return nil
}

// INSTRUCCIONES PARA OBTENER CHAT ID DE USUARIOS ESPECÍFICOS:
//
// 1. Cada usuario debe enviar un mensaje a tu bot
// 2. Usa este endpoint para ver los updates:
//    https://api.telegram.org/bot{TOKEN}/getUpdates
// 3. Busca el "chat_id" de cada usuario en la respuesta
// 4. Agrega el mapeo en phoneToTelegramMap arriba
//
// Ejemplo de respuesta:
// {
//   "result": [
//     {
//       "update_id": 123456,
//       "message": {
//         "message_id": 1,
//         "from": {
//           "id": 7521746198,  <- Este es el Chat ID
//           "first_name": "Hugo"
//         },
//         "chat": {
//           "id": 7521746198,  <- Chat ID (mismo que arriba para mensajes privados)
//         },
//         "text": "/start"
//       }
//     }
//   ]
// }
