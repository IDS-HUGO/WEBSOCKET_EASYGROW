package amqp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"WEBSOCKER_EASYGROW/internal/alerts"
	"WEBSOCKER_EASYGROW/internal/db"
	"WEBSOCKER_EASYGROW/internal/websocket"

	"github.com/streadway/amqp"
)

type SensorData struct {
	MacAddress string  `json:"mac_address"`
	Valor      float64 `json:"valor"`
	Nombre     string  `json:"nombre"`
}

func isCritical(sensor string, value float64) bool {
	switch strings.ToLower(sensor) {
	case "sensor de temperatura":
		return value > 35.0
	case "sensor de humedad":
		return value < 30.0
	case "sensor de luminosidad":
		return value < 50.0
	}
	return false
}

func getUserByMac(db *sql.DB, mac string) (email, phone string, err error) {
	// Primero verificar si el dispositivo existe
	var deviceExists bool
	checkDevice := `SELECT EXISTS(SELECT 1 FROM dispositivo WHERE mac_address = ?)`
	err = db.QueryRow(checkDevice, mac).Scan(&deviceExists)
	if err != nil {
		return "", "", fmt.Errorf("error verificando dispositivo: %w", err)
	}

	if !deviceExists {
		return "", "", fmt.Errorf("dispositivo con MAC %s no encontrado", mac)
	}

	// Verificar si el dispositivo tiene usuario asignado
	var userID sql.NullInt64
	getUserID := `SELECT id_usuario FROM dispositivo WHERE mac_address = ?`
	err = db.QueryRow(getUserID, mac).Scan(&userID)
	if err != nil {
		return "", "", fmt.Errorf("error obteniendo ID de usuario: %w", err)
	}

	if !userID.Valid {
		return "", "", fmt.Errorf("dispositivo con MAC %s no tiene usuario asignado", mac)
	}

	// Obtener datos del usuario
	query := `
		SELECT u.correo, u.telefono
		FROM usuarios u
		JOIN dispositivo d ON d.id_usuario = u.id_usuario
		WHERE d.mac_address = ?
	`

	err = db.QueryRow(query, mac).Scan(&email, &phone)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("no se encontró usuario para MAC %s", mac)
		}
		return "", "", fmt.Errorf("error en consulta SQL: %w", err)
	}

	// Verificar que los datos no estén vacíos
	if email == "" {
		return "", "", fmt.Errorf("usuario encontrado pero sin correo configurado")
	}
	if phone == "" {
		log.Printf("⚠️ Usuario encontrado pero sin teléfono configurado para MAC %s", mac)
	}

	return email, phone, nil
}

func ConsumeFromQueue(hub *websocket.Hub) {
	amqpURL := os.Getenv("AMQP_URL")
	queue := os.Getenv("QUEUE_NAME")

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatalf("❌ No se pudo conectar a RabbitMQ: %v", err)
	}
	defer conn.Close()
	log.Println("✅ Conexión a RabbitMQ OK")

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ Error abriendo canal: %v", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume(queue, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("❌ Error al consumir cola: %v", err)
	}

	dbConn, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("❌ BD error: %v", err)
	}
	defer dbConn.Close()

	for msg := range msgs {
		log.Printf("📥 Mensaje recibido: %s", msg.Body)
		hub.Broadcast(msg.Body)

		var data SensorData
		if err := json.Unmarshal(msg.Body, &data); err != nil {
			log.Printf("❌ Error al parsear JSON: %v", err)
			continue
		}

		// Debug: mostrar datos parseados
		log.Printf("🔍 Datos parseados - MAC: %s, Sensor: %s, Valor: %.2f",
			data.MacAddress, data.Nombre, data.Valor)

		if isCritical(data.Nombre, data.Valor) {
			log.Printf("🚨 Valor crítico detectado para MAC: %s", data.MacAddress)

			email, phone, err := getUserByMac(dbConn, data.MacAddress)
			if err != nil {
				log.Printf("❌ Error obteniendo usuario para MAC %s: %v", data.MacAddress, err)
				continue
			}

			log.Printf("✅ Usuario encontrado - Email: %s, Teléfono: %s", email, phone)

			alertMsg := fmt.Sprintf("🚨 ALERTA: %s con valor crítico: %.2f", data.Nombre, data.Valor)

			// Enviar alertas
			if email != "" {
				go func() {
					if err := alerts.SendEmailAlertTo(email, "⚠️ Alerta crítica en EasyGrow", alertMsg); err != nil {
						log.Printf("❌ Error enviando email: %v", err)
					} else {
						log.Printf("✅ Email enviado a: %s", email)
					}
				}()
			}

			if phone != "" {
				go func() {
					if err := alerts.SendWhatsAppAlertTo(phone, alertMsg); err != nil {
						log.Printf("❌ Error enviando WhatsApp: %v", err)
					} else {
						log.Printf("✅ WhatsApp enviado a: %s", phone)
					}
				}()

				go func() {
					if err := alerts.SendSMSAlertTo(phone, alertMsg); err != nil {
						log.Printf("❌ Error enviando SMS: %v", err)
					} else {
						log.Printf("✅ SMS enviado a: %s", phone)
					}
				}()
			}
		}
	}
}
