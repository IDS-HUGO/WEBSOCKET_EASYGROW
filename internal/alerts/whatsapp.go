package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type WhatsAppMessage struct {
	ChatId  string `json:"chatId"`
	Message string `json:"message"`
}

func SendWhatsAppAlert(phone, message string) error {
	apiInstance := os.Getenv("WHATSAPP_API_INSTANCE") // Ej: "1101842920"
	apiToken := os.Getenv("WHATSAPP_API_TOKEN")       // Ej: "yourGreenApiToken"
	url := fmt.Sprintf("https://api.green-api.com/waInstance%s/SendMessage/%s", apiInstance, apiToken)

	data := WhatsAppMessage{
		ChatId:  phone + "@c.us", // Número debe incluir LADA, ej: 5219612345678
		Message: message,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("error WhatsApp: Codigo %d", resp.StatusCode)
	}
	return nil
}
