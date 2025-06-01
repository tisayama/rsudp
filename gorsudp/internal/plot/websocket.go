package plot

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin in development
	},
	ReadBufferSize:  8192,  // Increased from 1024
	WriteBufferSize: 8192,  // Increased from 1024
}

// NewWebSocketManager creates a new WebSocket manager
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan WebSocketMessage, 1024), // Increased from 256
		stop:       make(chan struct{}),
		stopped:    false,
	}
}

// Start starts the WebSocket manager
func (manager *WebSocketManager) Start() {
	go manager.run()
}

// Stop stops the WebSocket manager
func (manager *WebSocketManager) Stop() {
	if manager.stopped {
		return
	}
	manager.stopped = true
	
	// Signal the run loop to stop
	close(manager.stop)
	
	// Close all client connections
	for client := range manager.clients {
		client.conn.Close()
		close(client.send)
	}
}

// run handles WebSocket manager operations
func (manager *WebSocketManager) run() {
	for {
		select {
		case <-manager.stop:
			log.Println("WebSocket manager stopping...")
			return
		case client := <-manager.register:
			manager.clients[client] = true
			log.Printf("WebSocket client registered. Total clients: %d", len(manager.clients))
			
			// Send welcome message with current configuration
			welcomeMsg := WebSocketMessage{
				Type: MessageTypeConfig,
				Data: map[string]interface{}{
					"status": "connected",
					"client_id": len(manager.clients),
				},
			}
			select {
			case client.send <- welcomeMsg:
			default:
				close(client.send)
				delete(manager.clients, client)
			}

		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				delete(manager.clients, client)
				close(client.send)
				log.Printf("❌ WebSocket client disconnected. Total clients: %d", len(manager.clients))
			}

		case message := <-manager.broadcast:
			// Broadcast message to all clients
			for client := range manager.clients {
				// Check if client is interested in this channel (if applicable)
				if manager.clientInterestedInMessage(client, message) {
					select {
					case client.send <- message:
					default:
						close(client.send)
						delete(manager.clients, client)
					}
				}
			}
		}
	}
}

// clientInterestedInMessage checks if a client should receive a message
func (manager *WebSocketManager) clientInterestedInMessage(client *Client, message WebSocketMessage) bool {
	// If client has no channel filters, send all messages
	if len(client.channels) == 0 {
		return true
	}

	// For plot data messages, check channel filters
	if message.Type == MessageTypePlotData || message.Type == MessageTypeSpectrogram {
		if data, ok := message.Data.(PlotData); ok {
			for _, channel := range client.channels {
				if channel == "all" || channel == data.Channel {
					return true
				}
			}
			return false
		}
	}

	// Send system messages to all clients
	return true
}

// Broadcast sends a message to all connected clients
func (manager *WebSocketManager) Broadcast(message WebSocketMessage) {
	if manager.stopped {
		return
	}
	select {
	case manager.broadcast <- message:
	default:
		log.Printf("⚠️ WebSocket broadcast channel full, dropping message of type: %s", message.Type)
	}
}

// GetClientCount returns the number of connected clients
func (manager *WebSocketManager) GetClientCount() int {
	return len(manager.clients)
}

// HandleWebSocket handles WebSocket upgrade requests
func (manager *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔌 WebSocket upgrade request from %s (User-Agent: %s)", r.RemoteAddr, r.UserAgent())
	
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("❌ WebSocket upgrade error from %s: %v", r.RemoteAddr, err)
		return
	}

	log.Printf("✅ WebSocket connection established from %s", r.RemoteAddr)

	// Parse channel filters from query parameters
	channels := parseChannelFilters(r)
	
	client := &Client{
		conn:     conn,
		send:     make(chan WebSocketMessage, 512), // Increased from 256
		channels: channels,
	}

	manager.register <- client

	// Start goroutines for this client
	go client.writePump(manager)
	go client.readPump(manager)
}

// parseChannelFilters extracts channel filters from HTTP request
func parseChannelFilters(r *http.Request) []string {
	channelsParam := r.URL.Query().Get("channels")
	if channelsParam == "" {
		return []string{"all"}
	}
	
	channels := strings.Split(channelsParam, ",")
	for i, channel := range channels {
		channels[i] = strings.TrimSpace(channel)
	}
	
	return channels
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump(manager *WebSocketManager) {
	ticker := time.NewTicker(30 * time.Second) // Send ping every 30 seconds
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				log.Printf("📤 WebSocket send channel closed for %s", c.conn.RemoteAddr())
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			err := c.conn.WriteJSON(message)
			if err != nil {
				log.Printf("🔥 WebSocket write error to %s: %v", c.conn.RemoteAddr(), err)
				return
			}
		
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("🏓 WebSocket ping error to %s: %v", c.conn.RemoteAddr(), err)
				return
			}
			log.Printf("🏓 Sent ping to %s", c.conn.RemoteAddr())
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *Client) readPump(manager *WebSocketManager) {
	defer func() {
		manager.unregister <- c
		c.conn.Close()
	}()

	// Set read deadline and pong handler
	c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		log.Printf("🏓 Received pong from %s", c.conn.RemoteAddr())
		c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		var msg map[string]interface{}
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("🔥 WebSocket unexpected close error from %s: %v", c.conn.RemoteAddr(), err)
			} else {
				log.Printf("📞 WebSocket client %s disconnected normally: %v", c.conn.RemoteAddr(), err)
			}
			break
		}

		// Handle incoming messages from client
		c.handleClientMessage(msg, manager)
	}
}

// handleClientMessage processes messages received from clients
func (c *Client) handleClientMessage(msg map[string]interface{}, manager *WebSocketManager) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "subscribe_channels":
		if data, ok := msg["data"].(map[string]interface{}); ok {
			if channelsRaw, ok := data["channels"].([]interface{}); ok {
				channels := make([]string, len(channelsRaw))
				for i, ch := range channelsRaw {
					if chStr, ok := ch.(string); ok {
						channels[i] = chStr
					}
				}
				c.channels = channels
				log.Printf("Client updated channel subscription: %v", channels)
			}
		}
	case "ping":
		// Respond to ping with pong
		pongMsg := WebSocketMessage{
			Type: "pong",
			Data: map[string]interface{}{
				"timestamp": msg["data"],
			},
		}
		select {
		case c.send <- pongMsg:
		default:
		}
	}
}