package alerts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Función principal que intenta múltiples servicios SMS GRATUITOS
func SendSMSAlertTo(phone, message string) error {
	// Intentar TextBelt primero (1 SMS gratis/día)
	if err := SendSMSTextBelt(phone, message); err == nil {
		return nil
	}

	// Intentar SMS Gateway ME
	if err := SendSMSGatewayMe(phone, message); err == nil {
		return nil
	}

	// Intentar SMS-API
	if err := SendSMSViaAPI(phone, message); err == nil {
		return nil
	}

	return fmt.Errorf("❌ Todos los servicios SMS fallaron")
}

// TextBelt - 1 SMS GRATIS por día
func SendSMSTextBelt(phone, message string) error {
	data := url.Values{}
	data.Set("phone", phone)
	data.Set("message", message)
	data.Set("key", "textbelt")

	resp, err := http.PostForm("https://textbelt.com/text", data)
	if err != nil {
		return fmt.Errorf("❌ Error TextBelt: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("❌ Error decodificando TextBelt: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		return fmt.Errorf("❌ TextBelt falló: %v", result["error"])
	}

	return nil
}

// SMS Gateway ME - GRATIS con app Android
func SendSMSGatewayMe(phone, message string) error {
	email := os.Getenv("SMSGATEWAY_EMAIL")
	password := os.Getenv("SMSGATEWAY_PASSWORD")
	deviceID := os.Getenv("SMSGATEWAY_DEVICE_ID")

	if email == "" || password == "" || deviceID == "" {
		return fmt.Errorf("❌ SMS Gateway ME no configurado")
	}

	// Login
	loginData := map[string]string{
		"email":    email,
		"password": password,
	}

	loginJSON, _ := json.Marshal(loginData)
	loginResp, err := http.Post("https://smsgateway.me/api/v4/user/login", "application/json",
		strings.NewReader(string(loginJSON)))
	if err != nil {
		return fmt.Errorf("❌ Login SMS Gateway ME: %w", err)
	}
	defer loginResp.Body.Close()

	var loginResult map[string]interface{}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)

	token, ok := loginResult["token"].(string)
	if !ok {
		return fmt.Errorf("❌ Token SMS Gateway ME inválido")
	}

	// Enviar SMS
	smsData := map[string]interface{}{
		"phone":   phone,
		"message": message,
		"device":  deviceID,
	}

	smsJSON, _ := json.Marshal(smsData)
	req, _ := http.NewRequest("POST", "https://smsgateway.me/api/v4/message/send",
		strings.NewReader(string(smsJSON)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error enviando SMS Gateway ME: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ SMS Gateway ME falló: %s", resp.Status)
	}

	return nil
}

// SMS-API - GRATIS con límites
func SendSMSViaAPI(phone, message string) error {
	token := os.Getenv("SMS_API_TOKEN")
	if token == "" {
		return fmt.Errorf("❌ SMS-API token no configurado")
	}

	data := map[string]interface{}{
		"phone":   phone,
		"message": message,
	}

	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", "https://api.sms-api.com/sms/send",
		strings.NewReader(string(jsonData)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error SMS-API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ SMS-API falló: %s", resp.Status)
	}

	return nil
}
