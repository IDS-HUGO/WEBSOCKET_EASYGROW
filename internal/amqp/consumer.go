package amqp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"WEBSOCKER_EASYGROW/internal/alerts"
	"WEBSOCKER_EASYGROW/internal/db"
	"WEBSOCKER_EASYGROW/internal/websocket"

	"github.com/streadway/amqp"
)

// Estructuras para diferentes tipos de JSON
type SensorData struct {
	MacAddress string  `json:"mac_address"`
	Valor      float64 `json:"valor"`
	Nombre     string  `json:"nombre"`
}

type BombaEvent struct {
	MacAddress      string  `json:"mac_address"`
	Evento          string  `json:"evento"`
	Bomba           string  `json:"bomba"`
	IDSensor        int     `json:"id_sensor,omitempty"`
	ValorHumedad    float64 `json:"valor_humedad,omitempty"`
	TiempoEncendida int     `json:"tiempo_encendida_seg,omitempty"`
}

// Funciones para insertar en BD
func insertSensorReading(dbConn *sql.DB, data SensorData) error {
	// 1. Obtener el ID del sensor basado en MAC y nombre
	var sensorID int
	querySensor := `
		SELECT s.id_sensor 
		FROM sensor_datos s 
		JOIN dispositivo d ON s.id_dispositivo = d.id_dispositivo 
		WHERE d.mac_address = ? AND s.nombre_sensor = ? AND s.activo = 1
	`

	err := dbConn.QueryRow(querySensor, data.MacAddress, data.Nombre).Scan(&sensorID)
	if err != nil {
		log.Printf("❌ Error obteniendo sensor para MAC %s, nombre %s: %v", data.MacAddress, data.Nombre, err)
		return err
	}

	// 2. Obtener la planta asociada al dispositivo (si existe)
	var plantaID sql.NullInt64
	queryPlanta := `
		SELECT p.id_planta 
		FROM planta p 
		JOIN dispositivo d ON p.id_dispositivo = d.id_dispositivo 
		WHERE d.mac_address = ? AND p.activa = 1 
		LIMIT 1
	`
	dbConn.QueryRow(queryPlanta, data.MacAddress).Scan(&plantaID)

	// 3. Determinar calidad del dato
	calidad := "bueno"
	if isCritical(data.Nombre, data.Valor) {
		calidad = "critico"
	} else if isWarning(data.Nombre, data.Valor) {
		calidad = "advertencia"
	}

	// 4. Insertar lectura
	insertQuery := `
		INSERT INTO lectura_datos (valor, id_sensor, id_planta, calidad_dato) 
		VALUES (?, ?, ?, ?)
	`

	_, err = dbConn.Exec(insertQuery, data.Valor, sensorID, plantaID, calidad)
	if err != nil {
		log.Printf("❌ Error insertando lectura: %v", err)
		return err
	}

	log.Printf("✅ Lectura insertada: Sensor %d, Valor %.2f, Calidad %s", sensorID, data.Valor, calidad)
	return nil
}

func insertBombaEvent(dbConn *sql.DB, event BombaEvent) error {
	insertQuery := `
		INSERT INTO eventos_bomba (mac_address, evento, bomba, id_sensor, valor_humedad, tiempo_encendida_seg) 
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := dbConn.Exec(insertQuery,
		event.MacAddress,
		event.Evento,
		event.Bomba,
		nullInt(event.IDSensor),
		nullFloat(event.ValorHumedad),
		nullInt(event.TiempoEncendida))

	if err != nil {
		log.Printf("❌ Error insertando evento bomba: %v", err)
		return err
	}

	log.Printf("✅ Evento bomba insertado: MAC %s, Bomba %s, Evento: %s",
		event.MacAddress, event.Bomba, event.Evento)
	return nil
}

func createAlert(dbConn *sql.DB, macAddress string, sensorName string, valor float64) {
	// Obtener planta asociada
	var plantaID int
	queryPlanta := `
		SELECT p.id_planta 
		FROM planta p 
		JOIN dispositivo d ON p.id_dispositivo = d.id_dispositivo 
		WHERE d.mac_address = ? AND p.activa = 1 
		LIMIT 1
	`

	err := dbConn.QueryRow(queryPlanta, macAddress).Scan(&plantaID)
	if err != nil {
		log.Printf("⚠️ No se encontró planta activa para MAC %s", macAddress)
		return
	}

	// Determinar tipo de alerta
	tipoAlerta := "temperatura"
	switch strings.ToLower(sensorName) {
	case "sensor de humedad":
		tipoAlerta = "humedad"
	case "sensor de luminosidad":
		tipoAlerta = "luz"
	case "sensor ultrasónico":
		tipoAlerta = "riego"
	}

	mensaje := fmt.Sprintf("Valor crítico detectado: %.2f en %s (Dispositivo: %s)",
		valor, sensorName, macAddress)

	insertQuery := `
		INSERT INTO alertas (id_planta, tipo_alerta, nivel, mensaje) 
		VALUES (?, ?, 'critico', ?)
	`

	_, err = dbConn.Exec(insertQuery, plantaID, tipoAlerta, mensaje)
	if err != nil {
		log.Printf("❌ Error insertando alerta: %v", err)
	} else {
		log.Printf("✅ Alerta creada para planta %d: %s", plantaID, mensaje)
	}
}

// Funciones auxiliares
func nullInt(value int) interface{} {
	if value == 0 {
		return nil
	}
	return value
}

func nullFloat(value float64) interface{} {
	if value == 0 {
		return nil
	}
	return value
}

func isCritical(sensor string, value float64) bool {
	switch strings.ToLower(sensor) {
	case "sensor de temperatura":
		return value > 35.0 || value < 5.0
	case "sensor de humedad":
		return value < 30.0 || value > 90.0
	case "sensor de luminosidad":
		return value < 50.0
	case "sensor ultrasónico":
		return value < 5.0
	case "sensor de lluvia yl-83":
		return value == 0 // Está lloviendo
	case "sensor de vibración sw-420":
		return value == 1 // Vibración detectada
	}
	return false
}

func isWarning(sensor string, value float64) bool {
	switch strings.ToLower(sensor) {
	case "sensor de temperatura":
		return (value > 30.0 && value <= 35.0) || (value < 10.0 && value >= 5.0)
	case "sensor de humedad":
		return (value < 40.0 && value >= 30.0) || (value > 80.0 && value <= 90.0)
	case "sensor de luminosidad":
		return value < 100.0 && value >= 50.0
	case "sensor ultrasónico":
		return value < 10.0 && value >= 5.0
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
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("no se encontró usuario para MAC %s", mac)
		}
		return "", "", fmt.Errorf("error en consulta SQL: %w", err)
	}

	return email, phone, nil
}

// Función principal del consumer
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

	log.Println("🔄 Esperando mensajes de la cola...")
	log.Println("=" + strings.Repeat("=", 60))

	for msg := range msgs {
		log.Println("📥 MENSAJE RECIBIDO:")
		log.Printf("   📋 Raw Data: %s", string(msg.Body))
		log.Printf("   🕐 Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))

		// Siempre enviar a WebSocket primero
		hub.Broadcast(msg.Body)
		log.Println("   📤 Enviado a WebSocket")

		// Procesar según el tipo de JSON
		var rawMessage map[string]interface{}
		if err := json.Unmarshal(msg.Body, &rawMessage); err != nil {
			log.Printf("   ❌ Error parseando JSON: %v", err)
			continue
		}

		// Detectar tipo de mensaje
		if _, hasEvento := rawMessage["evento"]; hasEvento {
			// Es un evento de bomba
			var bombaEvent BombaEvent
			if err := json.Unmarshal(msg.Body, &bombaEvent); err != nil {
				log.Printf("   ❌ Error parseando evento bomba: %v", err)
				continue
			}

			log.Printf("   🚰 EVENTO BOMBA: %s", bombaEvent.Evento)
			log.Printf("      MAC: %s, Bomba: %s", bombaEvent.MacAddress, bombaEvent.Bomba)

			// Insertar en BD
			if err := insertBombaEvent(dbConn, bombaEvent); err != nil {
				log.Printf("   ❌ Error insertando evento bomba: %v", err)
			}

		} else if hasValor := rawMessage["valor"]; hasValor != nil {
			// Es un dato de sensor
			var sensorData SensorData
			if err := json.Unmarshal(msg.Body, &sensorData); err != nil {
				log.Printf("   ❌ Error parseando sensor data: %v", err)
				continue
			}

			log.Printf("   📊 SENSOR DATA: %s = %.2f", sensorData.Nombre, sensorData.Valor)
			log.Printf("      MAC: %s", sensorData.MacAddress)

			// Insertar en BD
			if err := insertSensorReading(dbConn, sensorData); err != nil {
				log.Printf("   ❌ Error insertando sensor data: %v", err)
			}

			// Verificar si es crítico y crear alerta
			if isCritical(sensorData.Nombre, sensorData.Valor) {
				log.Printf("   🚨 VALOR CRÍTICO DETECTADO")

				// Crear alerta en BD
				createAlert(dbConn, sensorData.MacAddress, sensorData.Nombre, sensorData.Valor)

				// Obtener usuario y enviar notificaciones
				email, phone, err := getUserByMac(dbConn, sensorData.MacAddress)
				if err != nil {
					log.Printf("   ❌ Error obteniendo usuario: %v", err)
				} else {
					log.Printf("   👤 Usuario: %s, Tel: %s", email, phone)

					alertMsg := fmt.Sprintf(`🚨 <b>ALERTA CRÍTICA</b>
📍 <b>Dispositivo:</b> %s
📊 <b>Sensor:</b> %s
⚠️ <b>Valor:</b> %.2f
🕐 <b>Fecha:</b> %s

🔧 Revisa tu sistema EasyGrow inmediatamente`,
						sensorData.MacAddress, sensorData.Nombre, sensorData.Valor,
						time.Now().Format("2006-01-02 15:04:05"))

					// Enviar alertas (Telegram, Email, SMS, WhatsApp)
					go sendAllAlerts(email, phone, alertMsg)
				}
			}
		} else {
			log.Printf("   ⚠️ Tipo de mensaje no reconocido")
		}

		log.Println("   " + strings.Repeat("-", 58))
	}
}

func sendAllAlerts(email, phone, message string) {
	// Telegram
	if err := alerts.SendTelegramAlertToUser(phone, message); err != nil {
		log.Printf("❌ Error Telegram: %v", err)
	}

	// Email
	if email != "" {
		emailSubject := "🚨 ALERTA CRÍTICA - EasyGrow"
		emailBody := strings.ReplaceAll(message, "<b>", "")
		emailBody = strings.ReplaceAll(emailBody, "</b>", "")

		if err := alerts.SendEmailAlertTo(email, emailSubject, emailBody); err != nil {
			log.Printf("❌ Error Email: %v", err)
		}
	}

	// SMS
	if phone != "" {
		smsMsg := strings.ReplaceAll(message, "<b>", "")
		smsMsg = strings.ReplaceAll(smsMsg, "</b>", "")

		if err := alerts.SendSMSAlert(phone, smsMsg); err != nil {
			log.Printf("❌ Error SMS: %v", err)
		}
	}

	// WhatsApp
	if phone != "" {
		waMsg := strings.ReplaceAll(message, "<b>", "*")
		waMsg = strings.ReplaceAll(waMsg, "</b>", "*")

		if err := alerts.SendWhatsAppAlert(phone, waMsg); err != nil {
			log.Printf("❌ Error WhatsApp: %v", err)
		}
	}
}
