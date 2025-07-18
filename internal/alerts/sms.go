package alerts

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Función principal que intenta múltiples servicios SMS GRATUITOS
func SendSMSAlertTo(phone, message string) error {
	// Formatear número de teléfono
	formattedPhone := formatPhoneNumberSMS(phone)

	// Intentar TextBelt primero (1 SMS gratis/día)
	if err := SendSMSTextBelt(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar SMS Gateway ME
	if err := SendSMSGatewayMe(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar SMS-API
	if err := SendSMSViaAPI(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar CallMeBot SMS (GRATIS)
	if err := SendSMSCallMeBot(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar SMSGlobal (trial gratuito)
	if err := SendSMSGlobal(formattedPhone, message); err == nil {
		return nil
	}

	return fmt.Errorf("❌ Todos los servicios SMS fallaron")
}

// Función para formatear número de teléfono para SMS
func formatPhoneNumberSMS(phone string) string {
	// Remover espacios y caracteres especiales
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")

	// Para SMS, algunos servicios prefieren sin el +
	if strings.HasPrefix(phone, "+") {
		return phone
	}

	// Si no empieza con +, agregar código de país (México +52)
	if strings.HasPrefix(phone, "52") {
		return "+" + phone
	}

	return "+52" + phone
}

// TextBelt - 1 SMS GRATIS por día (MEJORADO)
func SendSMSTextBelt(phone, message string) error {
	// TextBelt no requiere el +
	phoneFormatted := strings.TrimPrefix(phone, "+")

	data := url.Values{}
	data.Set("phone", phoneFormatted)
	data.Set("message", message)
	data.Set("key", "textbelt")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm("https://textbelt.com/text", data)
	if err != nil {
		return fmt.Errorf("❌ Error TextBelt: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("❌ Error decodificando TextBelt: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		errorMsg := "desconocido"
		if errStr, ok := result["error"].(string); ok {
			errorMsg = errStr
		}
		return fmt.Errorf("❌ TextBelt falló: %s", errorMsg)
	}

	return nil
}

// SMS Gateway ME - GRATIS con app Android (MEJORADO)
func SendSMSGatewayMe(phone, message string) error {
	email := os.Getenv("SMSGATEWAY_EMAIL")
	password := os.Getenv("SMSGATEWAY_PASSWORD")
	deviceID := os.Getenv("SMSGATEWAY_DEVICE_ID")

	if email == "" || password == "" || deviceID == "" {
		return fmt.Errorf("❌ SMS Gateway ME no configurado")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// Login
	loginData := map[string]string{
		"email":    email,
		"password": password,
	}

	loginJSON, err := json.Marshal(loginData)
	if err != nil {
		return fmt.Errorf("❌ Error marshalling login: %w", err)
	}

	loginResp, err := client.Post("https://smsgateway.me/api/v4/user/login", "application/json",
		strings.NewReader(string(loginJSON)))
	if err != nil {
		return fmt.Errorf("❌ Login SMS Gateway ME: %w", err)
	}
	defer loginResp.Body.Close()

	loginBody, _ := io.ReadAll(loginResp.Body)

	var loginResult map[string]interface{}
	if err := json.Unmarshal(loginBody, &loginResult); err != nil {
		return fmt.Errorf("❌ Error decodificando login: %w", err)
	}

	token, ok := loginResult["token"].(string)
	if !ok || token == "" {
		return fmt.Errorf("❌ Token SMS Gateway ME inválido: %s", string(loginBody))
	}

	// Enviar SMS
	phoneFormatted := strings.TrimPrefix(phone, "+")

	smsData := map[string]interface{}{
		"phone":   phoneFormatted,
		"message": message,
		"device":  deviceID,
	}

	smsJSON, err := json.Marshal(smsData)
	if err != nil {
		return fmt.Errorf("❌ Error marshalling SMS: %w", err)
	}

	req, err := http.NewRequest("POST", "https://smsgateway.me/api/v4/message/send",
		strings.NewReader(string(smsJSON)))
	if err != nil {
		return fmt.Errorf("❌ Error creando request SMS: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error enviando SMS Gateway ME: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ SMS Gateway ME falló: %s - %s", resp.Status, string(body))
	}

	return nil
}

// SMS-API - GRATIS con límites (MEJORADO)
func SendSMSViaAPI(phone, message string) error {
	token := os.Getenv("SMS_API_TOKEN")
	if token == "" {
		return fmt.Errorf("❌ SMS-API token no configurado")
	}

	phoneFormatted := strings.TrimPrefix(phone, "+")

	data := map[string]interface{}{
		"phone":   phoneFormatted,
		"message": message,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("❌ Error marshalling JSON: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", "https://api.sms-api.com/sms/send",
		strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("❌ Error creando request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error SMS-API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ SMS-API falló: %s - %s", resp.Status, string(body))
	}

	return nil
}

// CallMeBot SMS - GRATIS (NUEVO)
func SendSMSCallMeBot(phone, message string) error {
	apiKey := os.Getenv("CALLMEBOT_SMS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("❌ CallMeBot SMS no configurado")
	}

	phoneFormatted := strings.TrimPrefix(phone, "+")

	apiURL := "https://api.callmebot.com/sms.php"

	data := url.Values{}
	data.Set("phone", phoneFormatted)
	data.Set("text", message)
	data.Set("apikey", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error CallMeBot SMS: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ CallMeBot SMS falló: %s - %s", resp.Status, string(body))
	}

	return nil
}

// SMSGlobal - Trial gratuito (NUEVO)
func SendSMSGlobal(phone, message string) error {
	user := os.Getenv("SMSGLOBAL_USER")
	password := os.Getenv("SMSGLOBAL_PASSWORD")

	if user == "" || password == "" {
		return fmt.Errorf("❌ SMSGlobal no configurado")
	}

	phoneFormatted := strings.TrimPrefix(phone, "+")

	apiURL := "https://api.smsglobal.com/http-api.php"

	data := url.Values{}
	data.Set("action", "sendsms")
	data.Set("user", user)
	data.Set("password", password)
	data.Set("from", "EasyGrow")
	data.Set("to", phoneFormatted)
	data.Set("text", message)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error SMSGlobal: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ SMSGlobal falló: %s - %s", resp.Status, string(body))
	}

	// Verificar si contiene "ERROR"
	if strings.Contains(string(body), "ERROR") {
		return fmt.Errorf("❌ SMSGlobal error: %s", string(body))
	}

	return nil
}
