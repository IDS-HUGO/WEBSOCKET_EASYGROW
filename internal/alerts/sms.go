package alerts

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
)

func SendSMSAlertTo(phone, message string) error {
	apiKey := os.Getenv("CALLMEBOT_SMS_KEY")

	apiURL := fmt.Sprintf("https://api.callmebot.com/sms.php?phone=%s&text=%s&apikey=%s",
		url.QueryEscape(phone),
		url.QueryEscape(message),
		url.QueryEscape(apiKey),
	)

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("❌ SMS error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ SMS fallo: %s", resp.Status)
	}
	return nil
}
