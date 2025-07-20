// internal/amqp/consumer.go - VERSIÓN CON LOGS COMPLETOS
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

	log.Println("🔄 Esperando mensajes de la cola...")
	log.Println("=" + strings.Repeat("=", 60))

	for msg := range msgs {
		// MOSTRAR TODO LO QUE LLEGA - INFORMACIÓN COMPLETA
		log.Println("📥 MENSAJE RECIBIDO:")
		log.Printf("   📋 Raw Data: %s", string(msg.Body))
		log.Printf("   🕐 Timestamp: %s", time.Now().Format("2006-01-02 15:04:05"))
		log.Printf("   📏 Tamaño: %d bytes", len(msg.Body))

		// Intentar parsear el JSON
		var data SensorData
		if err := json.Unmarshal(msg.Body, &data); err != nil {
			log.Printf("   ❌ Error al parsear JSON: %v", err)
			log.Println("   📤 Reenviando a WebSocket sin procesar...")
			hub.Broadcast(msg.Body)
			log.Println("   " + strings.Repeat("-", 58))
			continue
		}

		// MOSTRAR DATOS PARSEADOS CON DETALLE
		log.Println("   ✅ JSON parseado exitosamente:")
		log.Printf("   🔧 MAC Address: %s", data.MacAddress)
		log.Printf("   📊 Sensor: %s", data.Nombre)
		log.Printf("   📈 Valor: %.2f", data.Valor)

		// Verificar si es crítico
		critical := isCritical(data.Nombre, data.Valor)
		if critical {
			log.Printf("   🚨 ESTADO: CRÍTICO ⚠️")
		} else {
			log.Printf("   ✅ ESTADO: Normal")
		}

		// Siempre enviar a WebSocket
		log.Printf("JSON AL FRONT: %s", string(msg.Body))
		log.Println("   📤 Enviando a WebSocket...")
		hub.Broadcast(msg.Body)

		// Si es crítico, procesar alertas
		if critical {
			log.Println("   🔍 Procesando alerta crítica...")

			email, phone, err := getUserByMac(dbConn, data.MacAddress)
			if err != nil {
				log.Printf("   ❌ Error obteniendo usuario para MAC %s: %v", data.MacAddress, err)
				log.Println("   " + strings.Repeat("-", 58))
				continue
			}

			log.Printf("   👤 Usuario encontrado:")
			log.Printf("      📧 Email: %s", email)
			log.Printf("      📱 Teléfono: %s", phone)

			alertMsg := fmt.Sprintf(`🚨 <b>ALERTA CRÍTICA</b>
📍 <b>Dispositivo:</b> %s
📊 <b>Sensor:</b> %s
⚠️ <b>Valor:</b> %.2f
🕐 <b>Fecha:</b> %s

🔧 Revisa tu sistema EasyGrow inmediatamente`,
				data.MacAddress, data.Nombre, data.Valor, time.Now().Format("2006-01-02 15:04:05"))

			log.Println("   📬 Enviando alertas...")

			// 1. TELEGRAM - Principal y más confiable
			go func() {
				log.Printf("   📱 Enviando Telegram a: %s", phone)
				if err := alerts.SendTelegramAlertToUser(phone, alertMsg); err != nil {
					log.Printf("   ❌ Error Telegram: %v", err)
				} else {
					log.Printf("   ✅ Telegram enviado exitosamente")
				}
			}()

			// 2. EMAIL - Respaldo confiable
			go func() {
				if email != "" {
					log.Printf("   📧 Enviando Email a: %s", email)
					emailSubject := "🚨 ALERTA CRÍTICA - EasyGrow"
					emailBody := fmt.Sprintf(`
Hola,

Se ha detectado una alerta crítica en tu sistema EasyGrow:

🔸 Dispositivo: %s
🔸 Sensor: %s  
🔸 Valor registrado: %.2f
🔸 Hora: %s

Por favor, revisa tu sistema inmediatamente.

Saludos,
Equipo EasyGrow
					`, data.MacAddress, data.Nombre, data.Valor, time.Now().Format("2006-01-02 15:04:05"))

					if err := alerts.SendEmailAlertTo(email, emailSubject, emailBody); err != nil {
						log.Printf("   ❌ Error Email: %v", err)
					} else {
						log.Printf("   ✅ Email enviado exitosamente")
					}
				} else {
					log.Printf("   ⚠️ Email omitido (no configurado)")
				}
			}()

			// 3. SMS - Versión corregida con formato correcto
			go func(phone, alertMsg string) {
				if phone != "" {
					log.Printf("   📞 Enviando SMS a: %s", phone)
					// Limpiar mensaje para SMS (sin HTML)
					smsMsg := strings.ReplaceAll(alertMsg, "<b>", "")
					smsMsg = strings.ReplaceAll(smsMsg, "</b>", "")

					if err := alerts.SendSMSAlert(phone, smsMsg); err != nil {
						log.Printf("   ❌ Error SMS: %v", err)
					} else {
						log.Printf("   ✅ SMS enviado exitosamente")
					}
				} else {
					log.Printf("   ⚠️ SMS omitido (teléfono no configurado)")
				}
			}(phone, alertMsg)

			// 4. WHATSAPP - Versión corregida
			go func(phone, alertMsg string) {
				if phone != "" {
					log.Printf("   💬 Enviando WhatsApp a: %s", phone)
					// Limpiar mensaje para WhatsApp (sin HTML)
					waMsg := strings.ReplaceAll(alertMsg, "<b>", "*")
					waMsg = strings.ReplaceAll(waMsg, "</b>", "*")

					if err := alerts.SendWhatsAppAlert(phone, waMsg); err != nil {
						log.Printf("   ❌ Error WhatsApp: %v", err)
					} else {
						log.Printf("   ✅ WhatsApp enviado exitosamente")
					}
				} else {
					log.Printf("   ⚠️ WhatsApp omitido (teléfono no configurado)")
				}
			}(phone, alertMsg)

			log.Printf("   📤 Todas las alertas programadas para MAC: %s", data.MacAddress)
		} else {
			log.Println("   ℹ️ No se requiere alerta (valor normal)")
		}

		// Separador visual entre mensajes
		log.Println("   " + strings.Repeat("-", 58))
	}
}
