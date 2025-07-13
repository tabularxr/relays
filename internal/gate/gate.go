package gate

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/tabular/relay/pkg/types"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Gate manages WebSocket connections and message routing
type Gate struct {
	connections map[string]*types.Connection
	mutex       sync.RWMutex
	messageC    chan MessageEvent
	stopC       chan struct{}
	
	// Configuration
	bufferSize        int
	heartbeatInterval time.Duration
}

// MessageEvent wraps incoming messages with connection context
type MessageEvent struct {
	ConnectionID string
	Packet       types.StreamPacket
	Timestamp    time.Time
}

// New creates a new Gate instance
func New(bufferSize int, heartbeatInterval time.Duration) *Gate {
	return &Gate{
		connections:       make(map[string]*types.Connection),
		messageC:          make(chan MessageEvent, bufferSize),
		stopC:             make(chan struct{}),
		bufferSize:        bufferSize,
		heartbeatInterval: heartbeatInterval,
	}
}

// Start begins the gate operations
func (g *Gate) Start() {
	go g.heartbeatLoop()
}

// Stop gracefully shuts down the gate
func (g *Gate) Stop() {
	close(g.stopC)
}

// Messages returns the channel for incoming messages
func (g *Gate) Messages() <-chan MessageEvent {
	return g.messageC
}

// HandleWebSocket handles incoming WebSocket connections
func (g *Gate) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Validate API key
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		http.Error(w, "Missing API key", http.StatusUnauthorized)
		return
	}

	// Accept WebSocket connection
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Configure based on security needs
	})
	if err != nil {
		log.Printf("Failed to accept websocket: %v", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "Internal server error")

	// Create connection
	conn := &types.Connection{
		ID:        generateConnectionID(),
		LastSeen:  time.Now(),
		APIKey:    apiKey,
	}

	// Register connection
	g.addConnection(conn)
	defer g.removeConnection(conn.ID)

	log.Printf("WebSocket connection established: %s", conn.ID)

	// Handle messages
	ctx := context.Background()
	for {
		select {
		case <-g.stopC:
			return
		default:
			var packet types.StreamPacket
			err := wsjson.Read(ctx, c, &packet)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					log.Printf("WebSocket closed normally: %s", conn.ID)
				} else {
					log.Printf("WebSocket read error: %v", err)
				}
				return
			}

			// Update connection info
			if packet.SessionID != "" && conn.SessionID == "" {
				conn.SessionID = packet.SessionID
			}
			conn.LastSeen = time.Now()

			// Forward message
			select {
			case g.messageC <- MessageEvent{
				ConnectionID: conn.ID,
				Packet:       packet,
				Timestamp:    time.Now(),
			}:
			default:
				log.Printf("Message buffer full, dropping packet from %s", conn.ID)
			}
		}
	}
}

// GetActiveConnections returns the count of active connections
func (g *Gate) GetActiveConnections() int {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return len(g.connections)
}

// GetConnectionsBySession returns connections for a specific session
func (g *Gate) GetConnectionsBySession(sessionID string) []*types.Connection {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	
	var connections []*types.Connection
	for _, conn := range g.connections {
		if conn.SessionID == sessionID {
			connections = append(connections, conn)
		}
	}
	return connections
}

// addConnection registers a new connection
func (g *Gate) addConnection(conn *types.Connection) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.connections[conn.ID] = conn
}

// removeConnection unregisters a connection
func (g *Gate) removeConnection(id string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	delete(g.connections, id)
}

// heartbeatLoop periodically cleans up stale connections
func (g *Gate) heartbeatLoop() {
	ticker := time.NewTicker(g.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			g.cleanupStaleConnections()
		case <-g.stopC:
			return
		}
	}
}

// cleanupStaleConnections removes connections that haven't been seen recently
func (g *Gate) cleanupStaleConnections() {
	staleThreshold := time.Now().Add(-g.heartbeatInterval * 3)
	
	g.mutex.Lock()
	for id, conn := range g.connections {
		if conn.LastSeen.Before(staleThreshold) {
			log.Printf("Removing stale connection: %s", id)
			delete(g.connections, id)
		}
	}
	g.mutex.Unlock()
}

// generateConnectionID creates a unique connection identifier
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d", time.Now().UnixNano())
}