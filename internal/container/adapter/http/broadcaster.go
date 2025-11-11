package http

import (
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

type Client struct {
	ID   string
	Conn *websocket.Conn
	Send chan []byte
}

type Broadcaster struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

//nolint:cyclop
func (b *Broadcaster) Run() {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client.ID] = client
			b.mu.Unlock()
			log.Printf("WebSocket client registered: %s (total: %d)", client.ID, len(b.clients))

		case client := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[client.ID]; ok {
				delete(b.clients, client.ID)
				close(client.Send)
				log.Printf("WebSocket client unregistered: %s (total: %d)", client.ID, len(b.clients))
			}
			b.mu.Unlock()

		case message := <-b.broadcast:
			b.mu.RLock()
			for _, client := range b.clients {
				select {
				case client.Send <- message:
				default:
					b.mu.RUnlock()
					b.unregister <- client
					b.mu.RLock()
				}
			}
			b.mu.RUnlock()

		case <-heartbeatTicker.C:
			heartbeat := NewHeartbeatEvent()
			if data, err := heartbeat.ToJSON(); err == nil {
				b.Broadcast(data)
			}
		}
	}
}

func (b *Broadcaster) Register(client *Client) {
	b.register <- client
}

func (b *Broadcaster) Unregister(client *Client) {
	b.unregister <- client
}

func (b *Broadcaster) Broadcast(message []byte) {
	b.broadcast <- message
}

func (b *Broadcaster) BroadcastEvent(event *WebSocketEvent) error {
	data, err := event.ToJSON()
	if err != nil {
		return err
	}
	b.Broadcast(data)
	return nil
}

func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

func WritePump(client *Client, b *Broadcaster) {
	defer func() {
		b.Unregister(client)
		_ = client.Conn.Close()
	}()

	for message := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Error writing to WebSocket client %s: %v", client.ID, err)
			return
		}
	}

	_ = client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
}

func ReadPump(client *Client, b *Broadcaster) {
	defer func() {
		b.Unregister(client)
		_ = client.Conn.Close()
	}()

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for client %s: %v", client.ID, err)
			}
			break
		}
	}
}
