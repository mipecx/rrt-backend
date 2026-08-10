// Package ws содержит транспортный WebSocket-хаб, не привязанный к конкретному домену.
// Используется всеми доменами (rrt, incidents, ...) для рассылки real-time обновлений.
package ws

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients       map[*websocket.Conn]bool
	broadcast     chan []byte
	mu            sync.Mutex
	allowedOrigin map[string]bool
}

func NewHub(allowedOrigins []string) *Hub {
	originSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originSet[o] = true
	}
	return &Hub{
		clients:       make(map[*websocket.Conn]bool),
		broadcast:     make(chan []byte),
		allowedOrigin: originSet,
	}
}

func (h *Hub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	_, ok := h.allowedOrigin[origin]
	return ok
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
	upgrader := websocket.Upgrader{
		CheckOrigin: h.checkOrigin,
	}
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
