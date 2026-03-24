package wshub

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Hub struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]map[*websocket.Conn]struct{}
}

func New() *Hub {
	return &Hub{rooms: make(map[uuid.UUID]map[*websocket.Conn]struct{})}
}

func (h *Hub) Add(hallID uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[hallID] == nil {
		h.rooms[hallID] = make(map[*websocket.Conn]struct{})
	}
	h.rooms[hallID][c] = struct{}{}
}

func (h *Hub) Remove(hallID uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m, ok := h.rooms[hallID]; ok {
		delete(m, c)
		if len(m) == 0 {
			delete(h.rooms, hallID)
		}
	}
}

func (h *Hub) Broadcast(hallID uuid.UUID, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("ws marshal: %v", err)
		return
	}
	h.mu.RLock()
	var list []*websocket.Conn
	for c := range h.rooms[hallID] {
		list = append(list, c)
	}
	h.mu.RUnlock()
	for _, c := range list {
		_ = c.WriteMessage(websocket.TextMessage, data)
	}
}
