package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type VonageResponse struct {
	Messages []struct {
		Status           string `json:"status"`
		MessageID        string `json:"message-id"`
		ErrorText        string `json:"error-text"`
		RemainingBalance string `json:"remaining-balance"`
		MessagePrice     string `json:"message-price"`
	} `json:"messages"`
}

func SendSMSAlert(to, message string) error {
	apiKey := os.Getenv("VONAGE_API_KEY")
	apiSecret := os.Getenv("VONAGE_API_SECRET")
	from := os.Getenv("VONAGE_FROM_NUMBER")

	if apiKey == "" || apiSecret == "" {
		return fmt.Errorf("❌ Credenciales de Vonage no configuradas")
	}

	// API URL correcta para Vonage
	apiURL := "https://rest.nexmo.com/sms/json"

	// Preparar el payload JSON
	payload := map[string]string{
		"api_key":    apiKey,
		"api_secret": apiSecret,
		"to":         to,
		"from":       from,
		"text":       message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("❌ Error creando JSON: %w", err)
	}

	// Crear request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("❌ Error creando request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Cliente HTTP con timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Error enviando request: %w", err)
	}
	defer resp.Body.Close()

	// Leer respuesta
	var vonageResp VonageResponse
	if err := json.NewDecoder(resp.Body).Decode(&vonageResp); err != nil {
		return fmt.Errorf("❌ Error parseando respuesta: %w", err)
	}

	// Verificar respuesta
	if len(vonageResp.Messages) == 0 {
		return fmt.Errorf("❌ No se recibió respuesta de Vonage")
	}

	msg := vonageResp.Messages[0]
	if msg.Status == "0" {
		fmt.Printf("✅ SMS enviado exitosamente a: %s (ID: %s, Balance: %s)\n",
			to, msg.MessageID, msg.RemainingBalance)
		return nil
	}

	return fmt.Errorf("❌ Error Vonage: %s (Status: %s)", msg.ErrorText, msg.Status)
}
