package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.clients[conn] = true
		case conn := <-h.unregister:
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("❌ Error enviando mensaje: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleConnections(hub *Hub, w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ Error al hacer upgrade a WebSocket: %v", err)
		return
	}
	hub.register <- ws

	defer func() {
		hub.unregister <- ws
	}()
}
