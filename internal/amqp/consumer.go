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
	query := `
		SELECT u.correo, u.telefono
		FROM usuarios u
		JOIN dispositivo d ON d.id_usuario = u.id_usuario
		WHERE d.mac_address = ?
	`
	err = db.QueryRow(query, mac).Scan(&email, &phone)
	return
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

		if isCritical(data.Nombre, data.Valor) {
			email, phone, err := getUserByMac(dbConn, data.MacAddress)
			if err != nil {
				log.Printf("⚠️ Usuario no encontrado para MAC: %s", data.MacAddress)
				continue
			}

			alertMsg := fmt.Sprintf("🚨 ALERTA: %s con valor crítico: %.2f", data.Nombre, data.Valor)

			go alerts.SendEmailAlertTo(email, "⚠️ Alerta crítica en EasyGrow", alertMsg)
			go alerts.SendWhatsAppAlertTo(phone, alertMsg)
			go alerts.SendSMSAlertTo(phone, alertMsg)
		}
	}
}
