package main

import (
	"log"
	"net/http"
	"strings"

	"WEBSOCKER_EASYGROW/internal/amqp"
	"WEBSOCKER_EASYGROW/internal/websocket"
	"WEBSOCKER_EASYGROW/utils"
)

func main() {
	// Cargar variables de entorno
	utils.LoadEnv()

	// Crear el hub de WebSocket
	hub := websocket.NewHub()
	go hub.Run()

	// Iniciar el consumidor de múltiples colas en una goroutine
	go amqp.ConsumeFromQueues(hub)

	// Configurar el endpoint de WebSocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.HandleConnections(hub, w, r)
	})

	// Configurar endpoint de salud para verificar que el servicio esté corriendo
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "ok",
			"service": "EasyGrow WebSocket Consumer",
			"version": "2.0",
			"queues": ["datos_sensores", "eventos_bomba"]
		}`))
	})

	log.Println("🚀 Servidor EasyGrow WebSocket iniciado en puerto :8080")
	log.Println("📊 Consumiendo de cola: datos_sensores")
	log.Println("🚰 Consumiendo de cola: eventos_bomba")
	log.Println("🌐 WebSocket endpoint: ws://localhost:8080/ws")
	log.Println("=" + strings.Repeat("=", 50))
	log.Println("=" + strings.Repeat("=", 50))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
