package alerts

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
)

func SendWhatsAppAlertTo(phone, message string) error {
	apiKey := os.Getenv("CALLMEBOT_WHATSAPP_KEY")

	apiURL := fmt.Sprintf("https://api.callmebot.com/whatsapp.php?phone=%s&text=%s&apikey=%s",
		url.QueryEscape(phone),
		url.QueryEscape(message),
		url.QueryEscape(apiKey),
	)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("❌ WhatsApp error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ WhatsApp fallo: %s", resp.Status)
	}
	return nil
}
