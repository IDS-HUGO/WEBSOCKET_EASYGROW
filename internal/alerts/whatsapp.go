package alerts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Función principal que intenta múltiples servicios WhatsApp GRATUITOS
func SendWhatsAppAlertTo(phone, message string) error {
	// Intentar Green-API primero
	if err := SendWhatsAppGreenAPI(phone, message); err == nil {
		return nil
	}

	// Intentar UltraMsg
	if err := SendWhatsAppUltraMsg(phone, message); err == nil {
		return nil
	}

	// Intentar ChatAPI
	if err := SendWhatsAppChatAPI(phone, message); err == nil {
		return nil
	}

	return fmt.Errorf("❌ Todos los servicios WhatsApp fallaron")
}

// Green-API - GRATIS 3 días de prueba
func SendWhatsAppGreenAPI(phone, message string) error {
	instanceID := os.Getenv("GREEN_API_INSTANCE_ID")
	token := os.Getenv("GREEN_API_TOKEN")

	if instanceID == "" || token == "" {
		return fmt.Errorf("❌ Green-API no configurado")
	}

	apiURL := fmt.Sprintf("https://api.green-api.com/waInstance%s/sendMessage/%s", instanceID, token)

	data := map[string]interface{}{
		"chatId":  phone + "@c.us",
		"message": message,
	}

	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("❌ Error Green-API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ Green-API falló: %s", resp.Status)
	}

	return nil
}

// UltraMsg - GRATIS 1 día de prueba
func SendWhatsAppUltraMsg(phone, message string) error {
	instanceID := os.Getenv("ULTRAMSG_INSTANCE_ID")
	token := os.Getenv("ULTRAMSG_TOKEN")

	if instanceID == "" || token == "" {
		return fmt.Errorf("❌ UltraMsg no configurado")
	}

	apiURL := fmt.Sprintf("https://api.ultramsg.com/%s/messages/chat", instanceID)

	data := url.Values{}
	data.Set("token", token)
	data.Set("to", phone)
	data.Set("body", message)

	resp, err := http.PostForm(apiURL, data)
	if err != nil {
		return fmt.Errorf("❌ Error UltraMsg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ UltraMsg falló: %s", resp.Status)
	}

	return nil
}

// ChatAPI - GRATIS con límites
func SendWhatsAppChatAPI(phone, message string) error {
	token := os.Getenv("CHATAPI_TOKEN")
	if token == "" {
		return fmt.Errorf("❌ ChatAPI no configurado")
	}

	apiURL := fmt.Sprintf("https://api.chat-api.com/instance%s/sendMessage", token)

	data := map[string]interface{}{
		"phone": phone,
		"body":  message,
	}

	jsonData, _ := json.Marshal(data)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("❌ Error ChatAPI: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("❌ ChatAPI falló: %s", resp.Status)
	}

	return nil
}
