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

type WhatsAppMessage struct {
	ChatID  string `json:"chatId"`
	Message string `json:"message"`
}

func SendWhatsAppAlert(phone, message string) error {
	// Usar las variables correctas del .env
	apiInstance := os.Getenv("GREEN_API_INSTANCE_ID")
	apiToken := os.Getenv("GREEN_API_TOKEN")

	if apiInstance == "" || apiToken == "" {
		return fmt.Errorf("❌ Credenciales de Green API no configuradas")
	}

	// Verificar formato del número de teléfono
	if !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}

	// El número debe estar en formato internacional sin + y con @c.us
	// Ejemplo: +529727228805 -> 529727228805@c.us
	chatID := strings.TrimPrefix(phone, "+") + "@c.us"

	url := fmt.Sprintf("https://api.green-api.com/waInstance%s/SendMessage/%s", apiInstance, apiToken)

	data := WhatsAppMessage{
		ChatID:  chatID,
		Message: message,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("❌ Error creando JSON: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("❌ Error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error enviando request: %w", err)
	}
	defer resp.Body.Close()

	// Leer la respuesta para obtener más información del error
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err == nil {
		if resp.StatusCode == 403 {
			return fmt.Errorf("❌ WhatsApp Error 403: Posibles causas:\n"+
				"1. La instancia no está autorizada\n"+
				"2. El número no está en WhatsApp\n"+
				"3. Tu cuenta Green API no está activa\n"+
				"4. El número no está verificado en Green API\n"+
				"Respuesta: %v", response)
		}
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("❌ WhatsApp Error %d - Verifica tu cuenta Green API y que el número %s esté registrado", resp.StatusCode, phone)
	}

	fmt.Printf("✅ Mensaje WhatsApp enviado a: %s\n", phone)
	return nil
}
