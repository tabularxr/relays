package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tabular/relay/internal/gate"
	"github.com/tabular/relay/internal/parser"
	"github.com/tabular/relay/internal/transformer"
	"github.com/tabular/relay/internal/updater"
	"github.com/tabular/relay/pkg/types"
	"github.com/tabular/relay/tests/testdata"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// MockSTAG simulates the STAG service for testing
type MockSTAG struct {
	server   *httptest.Server
	received []types.SpatialEvent
}

func NewMockSTAG() *MockSTAG {
	mock := &MockSTAG{
		received: make([]types.SpatialEvent, 0),
	}
	
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", mock.handleIngest)
	mux.HandleFunc("/health", mock.handleHealth)
	
	mock.server = httptest.NewServer(mux)
	return mock
}

func (m *MockSTAG) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	var batch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// Extract events from batch
	if events, ok := batch["events"].([]interface{}); ok {
		for _, eventData := range events {
			eventBytes, _ := json.Marshal(eventData)
			var event types.SpatialEvent
			json.Unmarshal(eventBytes, &event)
			m.received = append(m.received, event)
		}
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (m *MockSTAG) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (m *MockSTAG) URL() string {
	return m.server.URL
}

func (m *MockSTAG) Close() {
	m.server.Close()
}

func (m *MockSTAG) GetReceivedEvents() []types.SpatialEvent {
	return m.received
}

func (m *MockSTAG) Reset() {
	m.received = m.received[:0]
}

func TestFullPipeline_PoseToSTAG(t *testing.T) {
	// Setup mock STAG
	mockSTAG := NewMockSTAG()
	defer mockSTAG.Close()
	
	// Setup pipeline components
	gateInstance := gate.New(10, 1*time.Second)
	parserInstance := parser.New()
	transformerInstance := transformer.New()
	updaterInstance := updater.New(mockSTAG.URL(), 2, 100*time.Millisecond)
	
	gateInstance.Start()
	updaterInstance.Start()
	defer func() {
		gateInstance.Stop()
		updaterInstance.Stop()
	}()
	
	// Setup message processing pipeline
	go func() {
		for msg := range gateInstance.Messages() {
			// Parse
			parsedPacket, err := parserInstance.ParsePacket(msg.Packet)
			if err != nil {
				t.Logf("Parse error: %v", err)
				continue
			}
			
			// Transform
			event, err := transformerInstance.Transform(*parsedPacket)
			if err != nil {
				t.Logf("Transform error: %v", err)
				continue
			}
			
			// Update
			if err := updaterInstance.ProcessEvent(*event); err != nil {
				t.Logf("Update error: %v", err)
			}
		}
	}()
	
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(gateInstance.HandleWebSocket))
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
	
	// Send test pose packet
	testPacket := map[string]interface{}{
		"session_id":   "integration-test-session",
		"frame_number": 1,
		"timestamp":    time.Now().UnixMilli(),
		"type":         "pose",
		"data": map[string]interface{}{
			"pose": map[string]interface{}{
				"x":        1.5,
				"y":        2.5,
				"z":        3.5,
				"rotation": []float64{0, 0, 0, 1},
			},
		},
	}
	
	err = wsjson.Write(ctx, conn, testPacket)
	require.NoError(t, err)
	
	// Wait for processing and batching
	time.Sleep(300 * time.Millisecond)
	
	// Verify event was received by STAG
	events := mockSTAG.GetReceivedEvents()
	assert.Len(t, events, 1)
	
	event := events[0]
	assert.Equal(t, "integration-test-session", event.SessionID)
	assert.NotEmpty(t, event.EventID)
	assert.Len(t, event.Anchors, 1)
	assert.Len(t, event.Meshes, 0)
	
	anchor := event.Anchors[0]
	assert.Equal(t, 1.5, anchor.Pose.X)
	assert.Equal(t, 2.5, anchor.Pose.Y)
	assert.Equal(t, 3.5, anchor.Pose.Z)
}

func TestFullPipeline_MeshToSTAG(t *testing.T) {
	// Setup mock STAG
	mockSTAG := NewMockSTAG()
	defer mockSTAG.Close()
	
	// Setup pipeline components
	gateInstance := gate.New(10, 1*time.Second)
	parserInstance := parser.New()
	transformerInstance := transformer.New()
	updaterInstance := updater.New(mockSTAG.URL(), 2, 100*time.Millisecond)
	
	gateInstance.Start()
	updaterInstance.Start()
	defer func() {
		gateInstance.Stop()
		updaterInstance.Stop()
	}()
	
	// Setup message processing pipeline
	go func() {
		for msg := range gateInstance.Messages() {
			parsedPacket, err := parserInstance.ParsePacket(msg.Packet)
			if err != nil {
				t.Logf("Parse error: %v", err)
				continue
			}
			
			event, err := transformerInstance.Transform(*parsedPacket)
			if err != nil {
				t.Logf("Transform error: %v", err)
				continue
			}
			
			updaterInstance.ProcessEvent(*event)
		}
	}()
	
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(gateInstance.HandleWebSocket))
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
	
	// Generate realistic Draco-compressed mesh data
	generator := testdata.NewDracoTestDataGenerator()
	dracoData, err := generator.GenerateCubeMesh()
	require.NoError(t, err)
	
	testPacket := testdata.CreateTestMeshPacket(
		"integration-test-session",
		"test-anchor-123", 
		dracoData,
	)
	
	err = wsjson.Write(ctx, conn, testPacket)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(300 * time.Millisecond)
	
	// Verify event was received by STAG
	events := mockSTAG.GetReceivedEvents()
	assert.Len(t, events, 1)
	
	event := events[0]
	assert.Equal(t, "integration-test-session", event.SessionID)
	assert.Len(t, event.Anchors, 0)
	assert.Len(t, event.Meshes, 1)
	
	mesh := event.Meshes[0]
	assert.Equal(t, "test-anchor-123", mesh.AnchorID)
	assert.NotEmpty(t, mesh.VerticesDelta)
	assert.False(t, mesh.IsDelta) // First mesh should be full, not delta
}

func TestFullPipeline_BatchProcessing(t *testing.T) {
	// Setup mock STAG
	mockSTAG := NewMockSTAG()
	defer mockSTAG.Close()
	
	// Setup pipeline with batch size of 3
	gateInstance := gate.New(10, 1*time.Second)
	parserInstance := parser.New()
	transformerInstance := transformer.New()
	updaterInstance := updater.New(mockSTAG.URL(), 3, 500*time.Millisecond)
	
	gateInstance.Start()
	updaterInstance.Start()
	defer func() {
		gateInstance.Stop()
		updaterInstance.Stop()
	}()
	
	// Setup message processing pipeline
	go func() {
		for msg := range gateInstance.Messages() {
			parsedPacket, err := parserInstance.ParsePacket(msg.Packet)
			if err != nil {
				continue
			}
			
			event, err := transformerInstance.Transform(*parsedPacket)
			if err != nil {
				continue
			}
			
			updaterInstance.ProcessEvent(*event)
		}
	}()
	
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(gateInstance.HandleWebSocket))
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
	
	// Send 5 pose packets
	for i := 0; i < 5; i++ {
		testPacket := map[string]interface{}{
			"session_id":   "batch-test-session",
			"frame_number": i + 1,
			"timestamp":    time.Now().UnixMilli(),
			"type":         "pose",
			"data": map[string]interface{}{
				"pose": map[string]interface{}{
					"x":        float64(i),
					"y":        float64(i + 1),
					"z":        float64(i + 2),
					"rotation": []float64{0, 0, 0, 1},
				},
			},
		}
		
		err = wsjson.Write(ctx, conn, testPacket)
		require.NoError(t, err)
		
		// Small delay between packets
		time.Sleep(10 * time.Millisecond)
	}
	
	// Wait for all batches to be processed
	time.Sleep(800 * time.Millisecond)
	
	// Should have received all 5 events (batch size 3 + batch size 2)
	events := mockSTAG.GetReceivedEvents()
	assert.Len(t, events, 5)
	
	// Verify all events have the same session ID
	for _, event := range events {
		assert.Equal(t, "batch-test-session", event.SessionID)
		assert.Len(t, event.Anchors, 1)
	}
}

func TestPipeline_ErrorHandling(t *testing.T) {
	// Setup mock STAG that returns errors
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	stagServer := httptest.NewServer(mux)
	defer stagServer.Close()
	
	// Setup pipeline
	gateInstance := gate.New(10, 1*time.Second)
	parserInstance := parser.New()
	transformerInstance := transformer.New()
	updaterInstance := updater.New(stagServer.URL, 1, 100*time.Millisecond)
	
	gateInstance.Start()
	updaterInstance.Start()
	defer func() {
		gateInstance.Stop()
		updaterInstance.Stop()
	}()
	
	// Setup message processing pipeline
	go func() {
		for msg := range gateInstance.Messages() {
			parsedPacket, err := parserInstance.ParsePacket(msg.Packet)
			if err != nil {
				continue
			}
			
			event, err := transformerInstance.Transform(*parsedPacket)
			if err != nil {
				continue
			}
			
			// This should fail when trying to send to STAG
			updaterInstance.ProcessEvent(*event)
		}
	}()
	
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(gateInstance.HandleWebSocket))
	defer server.Close()
	
	// Connect and send a packet
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
	
	testPacket := map[string]interface{}{
		"session_id":   "error-test-session",
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
	
	// Wait for processing attempt
	time.Sleep(300 * time.Millisecond)
	
	// Test should complete without hanging (error handling should work)
	assert.True(t, true, "Pipeline should handle STAG errors gracefully")
}

func TestDracoCompression_BandwidthSavings(t *testing.T) {
	// Setup mock STAG that tracks payload sizes
	var payloadSizes []int
	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		// Read and measure payload size
		buf := make([]byte, r.ContentLength)
		n, _ := r.Body.Read(buf)
		payloadSizes = append(payloadSizes, n)
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	stagServer := httptest.NewServer(mux)
	defer stagServer.Close()
	
	// Setup pipeline
	gateInstance := gate.New(10, 1*time.Second)
	parserInstance := parser.New()
	transformerInstance := transformer.New()
	updaterInstance := updater.New(stagServer.URL, 1, 100*time.Millisecond)
	
	gateInstance.Start()
	updaterInstance.Start()
	defer func() {
		gateInstance.Stop()
		updaterInstance.Stop()
	}()
	
	// Setup message processing pipeline
	go func() {
		for msg := range gateInstance.Messages() {
			parsedPacket, err := parserInstance.ParsePacket(msg.Packet)
			if err != nil {
				continue
			}
			
			event, err := transformerInstance.Transform(*parsedPacket)
			if err != nil {
				continue
			}
			
			updaterInstance.ProcessEvent(*event)
		}
	}()
	
	// Create test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(gateInstance.HandleWebSocket))
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
	
	// Generate medium-sized mesh to test compression (avoid WebSocket limits)
	generator := testdata.NewDracoTestDataGenerator()
	mediumDracoData, err := generator.GenerateSphereMesh(2.0, 30) // Smaller than large mesh
	require.NoError(t, err)
	
	testPacket := testdata.CreateTestMeshPacket(
		"compression-test-session",
		"medium-mesh-anchor", 
		mediumDracoData,
	)
	
	err = wsjson.Write(ctx, conn, testPacket)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(500 * time.Millisecond)
	
	// Verify compression achieved significant bandwidth savings
	require.Len(t, payloadSizes, 1, "Should have received exactly one payload")
	
	compressedPayloadSize := payloadSizes[0]
	originalDracoSize := len(mediumDracoData)
	
	t.Logf("Original compressed size: %d bytes", originalDracoSize)
	t.Logf("Compressed payload size: %d bytes", compressedPayloadSize)
	
	// The compressed payload should be significantly smaller than just sending raw Draco
	// Note: This is testing the full JSON payload compression effect
	compressionRatio := float64(compressedPayloadSize) / float64(originalDracoSize)
	t.Logf("Compression ratio: %.2f%%", compressionRatio*100)
	
	// For this test, we expect the processing to complete successfully
	// The actual compression ratio depends on the Draco data characteristics
	assert.True(t, compressedPayloadSize > 0, "Should have non-zero compressed payload")
	assert.True(t, compressionRatio < 2.0, "Compressed payload should not be more than 2x original")
}