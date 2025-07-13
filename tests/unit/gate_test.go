package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tabular/relay/internal/gate"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestGate_NewGate(t *testing.T) {
	g := gate.New(1024, 30*time.Second)
	
	assert.NotNil(t, g)
	assert.Equal(t, 0, g.GetActiveConnections())
}

func TestGate_WebSocketConnection(t *testing.T) {
	g := gate.New(10, 1*time.Second)
	g.Start()
	defer g.Stop()
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(g.HandleWebSocket))
	defer server.Close()
	
	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	
	// Test connection without API key (should fail)
	ctx := context.Background()
	_, _, err := websocket.Dial(ctx, wsURL, nil)
	assert.Error(t, err)
	
	// Test connection with API key
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-API-Key": []string{"test-key"},
		},
	}
	
	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")
	
	// Wait for connection to be registered
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, g.GetActiveConnections())
}

func TestGate_MessageProcessing(t *testing.T) {
	g := gate.New(10, 1*time.Second)
	g.Start()
	defer g.Stop()
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(g.HandleWebSocket))
	defer server.Close()
	
	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-API-Key": []string{"test-key"},
		},
	}
	
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")
	
	// Send test message
	testPacket := map[string]interface{}{
		"session_id":   "test-session",
		"frame_number": 1,
		"timestamp":    time.Now().UnixMilli(),
		"type":         "pose",
		"data": map[string]interface{}{
			"pose": map[string]interface{}{
				"x":        1.0,
				"y":        2.0,
				"z":        3.0,
				"rotation": []float64{0, 0, 0, 1},
			},
		},
	}
	
	err = wsjson.Write(ctx, conn, testPacket)
	require.NoError(t, err)
	
	// Read message from gate
	select {
	case msg := <-g.Messages():
		assert.Equal(t, "test-session", msg.Packet.SessionID)
		assert.Equal(t, "pose", msg.Packet.Type)
		assert.NotNil(t, msg.Packet.Data.Pose)
	case <-time.After(1 * time.Second):
		t.Fatal("Message not received within timeout")
	}
}

func TestGate_ConnectionCleanup(t *testing.T) {
	g := gate.New(10, 100*time.Millisecond) // Short heartbeat for testing
	g.Start()
	defer g.Stop()
	
	server := httptest.NewServer(http.HandlerFunc(g.HandleWebSocket))
	defer server.Close()
	
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-API-Key": []string{"test-key"},
		},
	}
	
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	require.NoError(t, err)
	
	// Wait for connection to be registered
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, g.GetActiveConnections())
	
	// Close connection
	conn.Close(websocket.StatusNormalClosure, "")
	
	// Wait for cleanup
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, 0, g.GetActiveConnections())
}

func TestGate_GetConnectionsBySession(t *testing.T) {
	g := gate.New(10, 1*time.Second)
	g.Start()
	defer g.Stop()
	
	server := httptest.NewServer(http.HandlerFunc(g.HandleWebSocket))
	defer server.Close()
	
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	opts := &websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-API-Key": []string{"test-key"},
		},
	}
	
	ctx := context.Background()
	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")
	
	// Send a message to establish session ID
	testPacket := map[string]interface{}{
		"session_id":   "test-session",
		"frame_number": 1,
		"timestamp":    time.Now().UnixMilli(),
		"type":         "pose",
		"data": map[string]interface{}{
			"pose": map[string]interface{}{
				"x":        1.0,
				"y":        2.0,
				"z":        3.0,
				"rotation": []float64{0, 0, 0, 1},
			},
		},
	}
	
	err = wsjson.Write(ctx, conn, testPacket)
	require.NoError(t, err)
	
	// Wait for message processing
	time.Sleep(10 * time.Millisecond)
	
	// Check session connections
	connections := g.GetConnectionsBySession("test-session")
	assert.Len(t, connections, 1)
	assert.Equal(t, "test-session", connections[0].SessionID)
}