package main

import (
	"log"
	"net/http"

	"WEBSOCKER_EASYGROW/internal/amqp"
	"WEBSOCKER_EASYGROW/internal/websocket"
	"WEBSOCKER_EASYGROW/utils"
)

func main() {
	utils.LoadEnv()

	hub := websocket.NewHub()
	go hub.Run()

	go amqp.ConsumeFromQueue(hub)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.HandleConnections(hub, w, r)
	})

	log.Println("🚀 Servidor WebSocket corriendo en :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))

}
