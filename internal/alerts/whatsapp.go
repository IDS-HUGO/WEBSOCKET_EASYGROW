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

// Función principal que intenta múltiples servicios WhatsApp GRATUITOS
func SendWhatsAppAlertTo(phone, message string) error {
	// Formatear número de teléfono
	formattedPhone := formatPhoneNumber(phone)

	// Intentar Green-API primero
	if err := SendWhatsAppGreenAPI(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar UltraMsg
	if err := SendWhatsAppUltraMsg(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar ChatAPI
	if err := SendWhatsAppChatAPI(formattedPhone, message); err == nil {
		return nil
	}

	// Intentar CallMeBot (GRATIS y sin registro)
	if err := SendWhatsAppCallMeBot(formattedPhone, message); err == nil {
		return nil
	}

	return fmt.Errorf("❌ Todos los servicios WhatsApp fallaron")
}

// Función para formatear número de teléfono
func formatPhoneNumber(phone string) string {
	// Remover espacios y caracteres especiales
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")

	// Si no empieza con +, agregar código de país (México +52)
	if !strings.HasPrefix(phone, "+") {
		if strings.HasPrefix(phone, "52") {
			phone = "+" + phone
		} else {
			phone = "+52" + phone
		}
	}

	return phone
}

// Green-API - GRATIS 3 días de prueba (MEJORADO)
func SendWhatsAppGreenAPI(phone, message string) error {
	instanceID := os.Getenv("GREEN_API_INSTANCE_ID")
	token := os.Getenv("GREEN_API_TOKEN")

	if instanceID == "" || token == "" {
		return fmt.Errorf("❌ Green-API no configurado")
	}

	apiURL := fmt.Sprintf("https://api.green-api.com/waInstance%s/sendMessage/%s", instanceID, token)

	// Formatear el chatId correctamente
	chatID := strings.TrimPrefix(phone, "+") + "@c.us"

	data := map[string]interface{}{
		"chatId":  chatID,
		"message": message,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("❌ Error marshalling JSON: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("❌ Error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error Green-API: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ Green-API falló: %s - %s", resp.Status, string(body))
	}

	// Verificar respuesta
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if idMessage, ok := result["idMessage"].(string); ok && idMessage != "" {
			return nil // Éxito
		}
	}

	return fmt.Errorf("❌ Green-API respuesta inválida: %s", string(body))
}

// UltraMsg - GRATIS 1 día de prueba (MEJORADO)
func SendWhatsAppUltraMsg(phone, message string) error {
	instanceID := os.Getenv("ULTRAMSG_INSTANCE_ID")
	token := os.Getenv("ULTRAMSG_TOKEN")

	if instanceID == "" || token == "" {
		return fmt.Errorf("❌ UltraMsg no configurado")
	}

	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/messages/chat", instanceID)

	// Formatear teléfono sin el +
	phoneFormatted := strings.TrimPrefix(phone, "+")

	data := url.Values{}
	data.Set("token", token)
	data.Set("to", phoneFormatted)
	data.Set("body", message)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error UltraMsg: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ UltraMsg falló: %s - %s", resp.Status, string(body))
	}

	return nil
}

// ChatAPI - GRATIS con límites (MEJORADO)
func SendWhatsAppChatAPI(phone, message string) error {
	token := os.Getenv("CHATAPI_TOKEN")
	if token == "" {
		return fmt.Errorf("❌ ChatAPI no configurado")
	}

	apiURL := fmt.Sprintf("https://api.chat-api.com/instance%s/sendMessage", token)

	// Formatear teléfono sin el +
	phoneFormatted := strings.TrimPrefix(phone, "+")

	data := map[string]interface{}{
		"phone": phoneFormatted,
		"body":  message,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("❌ Error marshalling JSON: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("❌ Error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error ChatAPI: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ ChatAPI falló: %s - %s", resp.Status, string(body))
	}

	return nil
}

// CallMeBot - GRATIS y sin registro (NUEVO)
func SendWhatsAppCallMeBot(phone, message string) error {
	// CallMeBot requiere que primero envíes un mensaje específico al bot
	// para obtener tu API key personal
	apiKey := os.Getenv("CALLMEBOT_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("❌ CallMeBot no configurado. Envía 'I allow callmebot to send me messages' a +34 644 59 71 67")
	}

	// Formatear teléfono sin el +
	phoneFormatted := strings.TrimPrefix(phone, "+")

	apiURL := "https://api.callmebot.com/whatsapp.php"

	data := url.Values{}
	data.Set("phone", phoneFormatted)
	data.Set("text", message)
	data.Set("apikey", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error CallMeBot: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ CallMeBot falló: %s - %s", resp.Status, string(body))
	}

	return nil
}
