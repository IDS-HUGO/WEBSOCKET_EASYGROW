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
	Fecha      string  `json:"fecha"`
}

type BombaEvent struct {
	MacAddress         string  `json:"mac_address"`
	Evento             string  `json:"evento"`
	Bomba              string  `json:"bomba,omitempty"`
	IDSensor           int     `json:"id_sensor,omitempty"`
	ValorHumedad       float64 `json:"valor_humedad,omitempty"`
	TiempoEncendidaSeg *int    `json:"tiempo_encendida_seg"`
	Fecha              string  `json:"fecha"`
}

// Función para insertar lecturas de sensores (mejorada)
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

	log.Printf("✅ Lectura insertada: Sensor %d (%s), Valor %.2f, Calidad %s",
		sensorID, data.Nombre, data.Valor, calidad)
	return nil
}

// Función para insertar evento de bomba (corregida)
func insertBombaEvent(dbConn *sql.DB, event BombaEvent) error {
	// 1. Verificar si el sensor existe antes de insertar
	var sensorExists bool
	if event.IDSensor != 0 {
		checkSensorQuery := `SELECT EXISTS(SELECT 1 FROM sensor_datos WHERE id_sensor = ? AND activo = 1)`
		err := dbConn.QueryRow(checkSensorQuery, event.IDSensor).Scan(&sensorExists)
		if err != nil {
			log.Printf("❌ Error verificando sensor ID %d: %v", event.IDSensor, err)
			return err
		}

		if !sensorExists {
			log.Printf("⚠️ ADVERTENCIA: Sensor ID %d no existe o está inactivo. Insertando evento sin referencia al sensor", event.IDSensor)
			// Insertar sin id_sensor para evitar el error de foreign key
			event.IDSensor = 0
		} else {
			log.Printf("✅ Sensor ID %d verificado correctamente", event.IDSensor)
		}
	}

	// 2. Extraer información de bomba del evento si no viene en el campo bomba
	bombaDetectada := event.Bomba
	if bombaDetectada == "" {
		eventoLower := strings.ToLower(event.Evento)
		if strings.Contains(eventoLower, "bomba a") {
			bombaDetectada = "A"
		} else if strings.Contains(eventoLower, "bomba b") {
			bombaDetectada = "B"
		}
	}

	// 3. Insertar el evento
	insertQuery := `
		INSERT INTO eventos_bomba (mac_address, evento, bomba, id_sensor, valor_humedad, tiempo_encendida_seg) 
		VALUES (?, ?, ?, ?, ?, ?)
	`

	var bomba sql.NullString
	if bombaDetectada != "" {
		bomba.String = bombaDetectada
		bomba.Valid = true
	}

	var idSensor sql.NullInt64
	if event.IDSensor != 0 && sensorExists {
		idSensor.Int64 = int64(event.IDSensor)
		idSensor.Valid = true
	}

	var valorHumedad sql.NullFloat64
	if event.ValorHumedad != 0 {
		valorHumedad.Float64 = event.ValorHumedad
		valorHumedad.Valid = true
	}

	var tiempoEncendida sql.NullInt64
	if event.TiempoEncendidaSeg != nil {
		tiempoEncendida.Int64 = int64(*event.TiempoEncendidaSeg)
		tiempoEncendida.Valid = true
	}

	_, err := dbConn.Exec(insertQuery,
		event.MacAddress,
		event.Evento,
		bomba,
		idSensor,
		valorHumedad,
		tiempoEncendida)

	if err != nil {
		log.Printf("❌ Error insertando evento bomba: %v", err)
		return err
	}

	log.Printf("✅ Evento bomba insertado: MAC %s, Bomba %s, Sensor ID %d, Evento: %s",
		event.MacAddress, bombaDetectada, event.IDSensor, event.Evento)
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
	sensorLower := strings.ToLower(sensorName)
	switch {
	case strings.Contains(sensorLower, "humedad") && !strings.Contains(sensorLower, "suelo") && !strings.Contains(sensorLower, "yl-69"):
		tipoAlerta = "humedad"
	case strings.Contains(sensorLower, "luminosidad"):
		tipoAlerta = "luz"
	case strings.Contains(sensorLower, "ultrasonico"):
		tipoAlerta = "riego"
	case strings.Contains(sensorLower, "lluvia") || strings.Contains(sensorLower, "yl-83"):
		tipoAlerta = "riego"
	case strings.Contains(sensorLower, "vibracion") || strings.Contains(sensorLower, "sw-420"):
		tipoAlerta = "riego" // Clasificamos vibración como riego por ahora
	case strings.Contains(sensorLower, "yl-69") || (strings.Contains(sensorLower, "humedad") && strings.Contains(sensorLower, "suelo")):
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

// Funciones auxiliares para detectar valores críticos (actualizadas)
func isCritical(sensor string, value float64) bool {
	sensor = strings.ToLower(sensor)
	switch {
	case strings.Contains(sensor, "temperatura"):
		return value > 35.0 || value < 5.0
	case strings.Contains(sensor, "humedad") && !strings.Contains(sensor, "suelo") && !strings.Contains(sensor, "yl-69"):
		return value < 30.0 || value > 90.0
	case strings.Contains(sensor, "luminosidad"):
		return value < 50.0
	case strings.Contains(sensor, "ultrasonico"):
		return value < 5.0 // Nivel de agua muy bajo
	case strings.Contains(sensor, "lluvia") || strings.Contains(sensor, "yl-83"):
		return value == 0 // Está lloviendo
	case strings.Contains(sensor, "vibracion") || strings.Contains(sensor, "sw-420"):
		return value == 1 // Vibración detectada
	case strings.Contains(sensor, "yl-69") || (strings.Contains(sensor, "humedad") && strings.Contains(sensor, "suelo")):
		// Para sensores YL-69: valores altos indican suelo seco (crítico)
		return value > 3000 // Suelo muy seco - necesita riego urgente
	}
	return false
}

func isWarning(sensor string, value float64) bool {
	sensor = strings.ToLower(sensor)
	switch {
	case strings.Contains(sensor, "temperatura"):
		return (value > 30.0 && value <= 35.0) || (value < 10.0 && value >= 5.0)
	case strings.Contains(sensor, "humedad") && !strings.Contains(sensor, "suelo") && !strings.Contains(sensor, "yl-69"):
		return (value < 40.0 && value >= 30.0) || (value > 80.0 && value <= 90.0)
	case strings.Contains(sensor, "luminosidad"):
		return value < 100.0 && value >= 50.0
	case strings.Contains(sensor, "ultrasonico"):
		return value < 10.0 && value >= 5.0 // Nivel de agua bajo
	case strings.Contains(sensor, "yl-69") || (strings.Contains(sensor, "humedad") && strings.Contains(sensor, "suelo")):
		// Advertencia si el suelo está empezando a secarse
		return value > 2000 && value <= 3000
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

// Consumer para la cola de datos de sensores
func consumeSensorData(ch *amqp.Channel, queueName string, dbConn *sql.DB, hub *websocket.Hub) {
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("❌ Error al consumir cola %s: %v", queueName, err)
	}

	log.Printf("🔄 Consumiendo mensajes de cola: %s", queueName)

	for msg := range msgs {
		log.Println("📥 SENSOR DATA RECIBIDO:")
		log.Printf("   📋 Raw Data: %s", string(msg.Body))
		log.Printf("   🕐 Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))

		// Enviar a WebSocket
		hub.Broadcast(msg.Body)
		log.Println("   📤 Enviado a WebSocket")

		// Procesar datos del sensor
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

				alertMsg := fmt.Sprintf(`🚨 <b>ALERTA CRÍTICA - SENSOR</b>
📍 <b>Dispositivo:</b> %s
📊 <b>Sensor:</b> %s
⚠️ <b>Valor:</b> %.2f
🕐 <b>Fecha:</b> %s

🔧 Revisa tu sistema EasyGrow inmediatamente`,
					sensorData.MacAddress, sensorData.Nombre, sensorData.Valor,
					time.Now().Format("2006-01-02 15:04:05"))

				// Enviar alertas
				go sendAllAlerts(email, phone, alertMsg)
			}
		}

		log.Println("   " + strings.Repeat("-", 58))
	}
}

// Consumer para la cola de eventos de bomba (corregido)
func consumeBombaEvents(ch *amqp.Channel, queueName string, dbConn *sql.DB, hub *websocket.Hub) {
	msgs, err := ch.Consume(queueName, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("❌ Error al consumir cola %s: %v", queueName, err)
	}

	log.Printf("🔄 Consumiendo mensajes de cola: %s", queueName)

	for msg := range msgs {
		log.Println("📥 BOMBA EVENT RECIBIDO:")
		log.Printf("   📋 Raw Data: %s", string(msg.Body))
		log.Printf("   🕐 Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))

		// Enviar a WebSocket
		hub.Broadcast(msg.Body)
		log.Println("   📤 Enviado a WebSocket")

		// Procesar evento de bomba
		var bombaEvent BombaEvent
		if err := json.Unmarshal(msg.Body, &bombaEvent); err != nil {
			log.Printf("   ❌ Error parseando evento bomba: %v", err)
			continue
		}

		// Extraer bomba del evento si no viene en el campo bomba
		bombaDetectada := bombaEvent.Bomba
		if bombaDetectada == "" {
			eventoLower := strings.ToLower(bombaEvent.Evento)
			if strings.Contains(eventoLower, "bomba a") {
				bombaDetectada = "A"
			} else if strings.Contains(eventoLower, "bomba b") {
				bombaDetectada = "B"
			}
		}

		log.Printf("   🚰 EVENTO BOMBA: %s", bombaEvent.Evento)
		log.Printf("      MAC: %s, Bomba: %s, Sensor ID: %d", bombaEvent.MacAddress, bombaDetectada, bombaEvent.IDSensor)
		if bombaEvent.ValorHumedad != 0 {
			log.Printf("      Valor Humedad YL-69: %.0f ADC", bombaEvent.ValorHumedad)
		}
		if bombaEvent.TiempoEncendidaSeg != nil {
			log.Printf("      Tiempo encendida: %d seg", *bombaEvent.TiempoEncendidaSeg)
		}

		// Insertar en BD
		if err := insertBombaEvent(dbConn, bombaEvent); err != nil {
			log.Printf("   ❌ Error insertando evento bomba: %v", err)
		}

		// Crear alerta informativa para eventos de bomba activada
		if strings.Contains(strings.ToLower(bombaEvent.Evento), "activada") {
			log.Printf("   💧 BOMBA ACTIVADA - Creando alerta informativa")

			// Obtener usuario y enviar notificación
			email, phone, err := getUserByMac(dbConn, bombaEvent.MacAddress)
			if err != nil {
				log.Printf("   ❌ Error obteniendo usuario: %v", err)
			} else {
				log.Printf("   👤 Usuario: %s, Tel: %s", email, phone)

				alertMsg := fmt.Sprintf(`💧 <b>BOMBA ACTIVADA</b>
📍 <b>Dispositivo:</b> %s
🚰 <b>Bomba:</b> %s
📊 <b>Sensor YL-69:</b> %.0f ADC (suelo seco)
🕐 <b>Fecha:</b> %s

💡 Tu sistema de riego está funcionando correctamente`,
					bombaEvent.MacAddress, bombaDetectada, bombaEvent.ValorHumedad,
					time.Now().Format("2006-01-02 15:04:05"))

				// Enviar solo notificación por Telegram (menos invasivo)
				go func() {
					if err := alerts.SendTelegramAlertToUser(phone, alertMsg); err != nil {
						log.Printf("❌ Error Telegram: %v", err)
					}
				}()
			}
		}

		log.Println("   " + strings.Repeat("-", 58))
	}
}

// Función principal del consumer - maneja dos colas
func ConsumeFromQueues(hub *websocket.Hub) {
	amqpURL := os.Getenv("AMQP_URL")
	sensorQueue := os.Getenv("SENSOR_QUEUE_NAME") // datos_sensores
	bombaQueue := os.Getenv("BOMBA_QUEUE_NAME")   // eventos_bomba

	// Verificar que las variables estén configuradas
	if sensorQueue == "" {
		sensorQueue = "datos_sensores" // valor por defecto
	}
	if bombaQueue == "" {
		bombaQueue = "eventos_bomba" // valor por defecto
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatalf("❌ No se pudo conectar a RabbitMQ: %v", err)
	}
	defer conn.Close()
	log.Println("✅ Conexión a RabbitMQ OK")

	// Crear canales separados para cada cola
	chSensor, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ Error abriendo canal para sensores: %v", err)
	}
	defer chSensor.Close()

	chBomba, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ Error abriendo canal para bombas: %v", err)
	}
	defer chBomba.Close()

	// Conectar a la base de datos
	dbConn, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("❌ BD error: %v", err)
	}
	defer dbConn.Close()

	log.Println("🔄 Iniciando consumidores para ambas colas...")
	log.Printf("   📊 Cola sensores: %s", sensorQueue)
	log.Printf("   🚰 Cola bombas: %s", bombaQueue)
	log.Println("=" + strings.Repeat("=", 60))

	// Lanzar goroutines para consumir de ambas colas simultáneamente
	go consumeSensorData(chSensor, sensorQueue, dbConn, hub)
	go consumeBombaEvents(chBomba, bombaQueue, dbConn, hub)

	// Mantener el programa corriendo
	select {}
}

// Función de compatibilidad - mantener para no romper el main.go existente
func ConsumeFromQueue(hub *websocket.Hub) {
	ConsumeFromQueues(hub)
}

func sendAllAlerts(email, phone, message string) {
	// Telegram (más confiable y gratuito)
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

	// SMS (solo para alertas críticas)
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
