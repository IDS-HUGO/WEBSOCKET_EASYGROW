package alerts

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func SendSMSAlert(to, message string) error {
	apiKey := os.Getenv("VONAGE_API_KEY")
	apiSecret := os.Getenv("VONAGE_API_SECRET")
	from := os.Getenv("VONAGE_FROM_NUMBER") // tu número virtual comprado o nombre alfanumérico

	apiURL := "https://rest.nexmo.com/sms/json"

	form := url.Values{}
	form.Set("api_key", apiKey)
	form.Set("api_secret", apiSecret)
	form.Set("to", to)
	form.Set("from", from)
	form.Set("text", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println("✅ SMS enviado a:", to)
		return nil
	}

	return fmt.Errorf("❌ Error al enviar SMS: status code %d", resp.StatusCode)
}
