package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yourorg/aggregator/internal/buffer"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Subscription defines what metrics a client wants to receive
type Subscription struct {
	Service string `json:"service"`
	Metric  string `json:"metric"`
}

// Client represents a WebSocket client connection
type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	send  chan []byte
	subs  []Subscription
	subMu sync.RWMutex
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	registry   *buffer.Registry
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	updates    chan string
	mu         sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub(registry *buffer.Registry) *Hub {
	return &Hub{
		registry:   registry,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		updates:    make(chan string, 1000),
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected. Total: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected. Total: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// StartBroadcastLoop starts the periodic broadcast loop
func (h *Hub) StartBroadcastLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		h.broadcastSnapshot()
	}
}

// broadcastSnapshot sends current metrics to all subscribed clients
func (h *Hub) broadcastSnapshot() {
	snapshot := h.registry.LatestSnapshot()

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		msg := h.buildClientMessage(client, snapshot)
		if msg != nil {
			select {
			case client.send <- msg:
			default:
				// Skip if buffer full
			}
		}
	}
}

// buildClientMessage creates a message for a specific client based on subscriptions
func (h *Hub) buildClientMessage(client *Client, snapshot buffer.LatestSnapshot) []byte {
	client.subMu.RLock()
	defer client.subMu.RUnlock()

	if len(client.subs) == 0 {
		// No subscriptions, send all
		data, _ := json.Marshal(map[string]interface{}{
			"type":       "snapshot",
			"timestamp":  time.Now().UnixNano(),
			"gauges":     convertGauges(snapshot.Gauges),
			"counters":   convertCounters(snapshot.Counters),
			"histograms": convertHistograms(snapshot.Histograms),
		})
		return data
	}

	// Filter by subscriptions
	gauges := make(map[string]interface{})
	counters := make(map[string]interface{})
	histograms := make(map[string]interface{})

	for _, sub := range client.subs {
		key := buffer.MetricKey{Service: sub.Service, Name: sub.Metric}
		if g, ok := snapshot.Gauges[key]; ok {
			gauges[key.String()] = map[string]interface{}{
				"ts":  g.Ts,
				"val": g.Val,
			}
		}
		if c, ok := snapshot.Counters[key]; ok {
			counters[key.String()] = map[string]interface{}{
				"ts":  c.Ts,
				"val": c.Val,
			}
		}
		if hist, ok := snapshot.Histograms[key]; ok {
			histograms[key.String()] = map[string]interface{}{
				"ts":     hist.Ts,
				"bounds": hist.Bounds,
				"counts": hist.Counts,
			}
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"type":       "snapshot",
		"timestamp":  time.Now().UnixNano(),
		"gauges":     gauges,
		"counters":   counters,
		"histograms": histograms,
	})
	return data
}

func convertGauges(gauges map[buffer.MetricKey]buffer.Sample) map[string]interface{} {
	result := make(map[string]interface{})
	for key, sample := range gauges {
		result[key.String()] = map[string]interface{}{
			"ts":  sample.Ts,
			"val": sample.Val,
		}
	}
	return result
}

func convertCounters(counters map[buffer.MetricKey]buffer.Sample) map[string]interface{} {
	result := make(map[string]interface{})
	for key, sample := range counters {
		result[key.String()] = map[string]interface{}{
			"ts":  sample.Ts,
			"val": sample.Val,
		}
	}
	return result
}

func convertHistograms(histograms map[buffer.MetricKey]buffer.HistogramData) map[string]interface{} {
	result := make(map[string]interface{})
	for key, hist := range histograms {
		result[key.String()] = map[string]interface{}{
			"ts":     hist.Ts,
			"bounds": hist.Bounds,
			"counts": hist.Counts,
		}
	}
	return result
}

// NotifyUpdate signals that new data is available for a service
func (h *Hub) NotifyUpdate(service string) {
	select {
	case h.updates <- service:
	default:
		// Channel full, skip notification
	}
}

// Broadcast sends a message to all clients
func (h *Hub) Broadcast(data []byte) {
	h.broadcast <- data
}

// HandleWebSocket handles new WebSocket connections
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
		subs: []Subscription{},
	}

	h.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump handles incoming messages from client
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle subscription messages
		var msg struct {
			Type string         `json:"type"`
			Subs []Subscription `json:"subscriptions"`
		}
		if err := json.Unmarshal(message, &msg); err == nil && msg.Type == "subscribe" {
			c.subMu.Lock()
			c.subs = msg.Subs
			c.subMu.Unlock()
			log.Printf("Client subscribed to %d metrics", len(msg.Subs))
		}
	}
}

// writePump handles outgoing messages to client
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch pending messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte("\n"))
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
