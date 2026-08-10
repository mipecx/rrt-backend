// Package ws содержит транспортный WebSocket-хаб, не привязанный к конкретному домену.
// Используется всеми доменами (rrt, incidents, ...) для рассылки real-time обновлений.
package ws

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Разрешаем CORS для дева
	},
}

type Hub struct {
	clients   map[*websocket.Conn]bool
	broadcast chan []byte
	mu        sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte),
	}
}

func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mu.Lock()
		for client := range h.clients {
			if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
				client.Close()
				delete(h.clients, client)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

// Broadcast — публичный вход для любого домена, который хочет разослать сообщение всем клиентам.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}
