package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 54 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // same-origin + JWT-gated
	},
}

type Client struct {
	hub    *Hub
	ws     *websocket.Conn
	user   *User
	send   chan []byte
}

type Hub struct {
	mu       sync.RWMutex
	clients  map[*Client]bool
	rooms    map[int64]map[*Client]bool
	presence map[int64]bool
	store    *Store
}

func newHub(store *Store) *Hub {
	return &Hub{
		clients:  make(map[*Client]bool),
		rooms:    make(map[int64]map[*Client]bool),
		presence: make(map[int64]bool),
		store:    store,
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.presence[c.user.ID] = true
	h.mu.Unlock()
	h.addToRoom(c, h.store.lobbyID)
	h.broadcastPresence(c.user, true)
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		h.presence[c.user.ID] = false
	}
	// remove from all rooms (never close c.send: senders drop messages instead)
	for _, room := range h.rooms {
		delete(room, c)
	}
	h.mu.Unlock()
	_ = h.store.SetLastOnline(context.Background(), c.user.ID)
	h.broadcastPresence(c.user, false)
}

func (h *Hub) addToRoom(c *Client, convID int64) {
	h.mu.Lock()
	room := h.rooms[convID]
	if room == nil {
		room = make(map[*Client]bool)
		h.rooms[convID] = room
	}
	room[c] = true
	h.mu.Unlock()
}

func (h *Hub) broadcastPresence(u *User, online bool) {
	payload := map[string]any{
		"type": "presence",
		"user": map[string]any{
			"id":       u.ID,
			"username": u.Username,
			"online":   online,
		},
	}
	h.sendToRoom(h.store.lobbyID, payload, nil)
}

func (h *Hub) isOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.presence[userID]
}

func (h *Hub) sendToRoom(convID int64, payload any, skip *Client) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.RLock()
	room := h.rooms[convID]
	clients := make([]*Client, 0, len(room))
	for c := range room {
		if c != skip {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		select {
		case c.send <- b:
		default: // slow/offline client: drop rather than block the hub
		}
	}
}

// handleWebSocket upgrades a connection and starts read/write pumps.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	userID, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := s.store.ensureLobbyMembership(r.Context(), userID); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("ws upgrade:", err)
		return
	}
	c := &Client{hub: s.hub, ws: conn, user: u, send: make(chan []byte, 64)}
	s.hub.register(c)
	go c.writePump()
	c.readPump()
}

type envelope struct {
	Type           string `json:"type"`
	ConversationID int64  `json:"conversation_id"`
	Content        string `json:"content"`
	FileURL        string `json:"file_url"`
	MsgType        string `json:"msg_type"`
	IsTyping       *bool  `json:"is_typing,omitempty"`
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.ws.Close()
	}()
	c.ws.SetReadLimit(1 << 20)
	c.ws.SetReadDeadline(time.Now().Add(wsPongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		var env envelope
		if err := c.ws.ReadJSON(&env); err != nil {
			return
		}
		switch env.Type {
		case "join":
			if !c.isMember(env.ConversationID) {
				c.sendErr("not a member of that conversation")
				continue
			}
			c.hub.addToRoom(c, env.ConversationID)

		case "leave":
			c.hub.mu.Lock()
			if room := c.hub.rooms[env.ConversationID]; room != nil {
				delete(room, c)
			}
			c.hub.mu.Unlock()

		case "message":
			if !c.isMember(env.ConversationID) {
				c.sendErr("not a member of that conversation")
				continue
			}
			if len(env.Content) > 10000 {
				c.sendErr("message too long")
				continue
			}
			mtype := env.MsgType
			if mtype == "" {
				mtype = "text"
			}
			if !messageTypes[mtype] {
				c.sendErr("unsupported message type")
				continue
			}
			if mtype == "text" && env.Content == "" {
				c.sendErr("empty message")
				continue
			}
			if mtype != "text" && env.FileURL == "" {
				c.sendErr("missing file")
				continue
			}
			m, err := c.hub.store.insertMessage(context.Background(), env.ConversationID, c.user.ID, mtype, env.Content, env.FileURL)
			if err != nil {
				log.Println("insert message:", err)
				c.sendErr("could not save message")
				continue
			}
			c.hub.sendToRoom(env.ConversationID, map[string]any{"type": "message", "message": m}, nil)

		case "typing":
			if !c.isMember(env.ConversationID) {
				continue
			}
			status := false
			if env.IsTyping != nil {
				status = *env.IsTyping
			}
			c.hub.sendToRoom(env.ConversationID, map[string]any{
				"type":           "typing",
				"conversation_id": env.ConversationID,
				"sender_username": c.user.Username,
				"is_typing":       status,
			}, c)
		}
	}
}

func (c *Client) isMember(convID int64) bool {
	ids, err := c.hub.store.conversationMembers(context.Background(), convID)
	if err != nil {
		return false
	}
	for _, id := range ids {
		if id == c.user.ID {
			return true
		}
	}
	return false
}

func (c *Client) sendErr(detail string) {
	b, _ := json.Marshal(map[string]any{"type": "error", "detail": detail})
	select {
	case c.send <- b:
	default:
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case b, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, b); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}