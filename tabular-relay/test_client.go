package main

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pierrec/lz4/v4"
)

// TestConfig holds configuration for the test client
type TestConfig struct {
	RelayURL     string
	StagsURL     string
	PacketFiles  []string
	Concurrent   int
	SendInterval time.Duration
	Verbose      bool
}

// TestResults holds the results of the test run
type TestResults struct {
	PacketsSent     int
	PacketsAcked    int
	Errors          []string
	StagsResponses  int
	StartTime       time.Time
	EndTime         time.Time
	mutex           sync.Mutex
}

// MockStagsServer represents a mock Stags ingest server
type MockStagsServer struct {
	server       *http.Server
	port         int
	responses    []StagsResponse
	mutex        sync.RWMutex
}

// StagsResponse represents a response from the mock Stags server
type StagsResponse struct {
	Timestamp   time.Time   `json:"timestamp"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	Headers     http.Header `json:"headers"`
	Body        string      `json:"body"`
	StatusCode  int         `json:"status_code"`
}

// PacketData represents the JSON structure of test packets
type PacketData struct {
	Header  PacketHeader  `json:"header"`
	Streams []StreamData  `json:"streams"`
}

type PacketHeader struct {
	Magic       string `json:"magic"`
	Version     uint16 `json:"version"`
	Timestamp   int64  `json:"timestamp"`
	FrameNumber uint64 `json:"frame_number"`
	SessionID   string `json:"session_id"`
	ClientID    string `json:"client_id"`
	StreamCount uint32 `json:"stream_count"`
	TotalSize   uint32 `json:"total_size"`
}

type StreamData struct {
	Metadata StreamMetadata     `json:"metadata"`
	Data     interface{}        `json:"data"`
}

type StreamMetadata struct {
	Type           string                 `json:"type"`
	Size           uint32                 `json:"size"`
	CompressedSize uint32                 `json:"compressed_size"`
	Compression    string                 `json:"compression"`
	Timestamp      int64                  `json:"timestamp"`
	SequenceNumber uint32                 `json:"sequence_number"`
	Extras         map[string]interface{} `json:"extras,omitempty"`
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Start mock Stags server
	mockStags := startMockStagsServer(8000)
	defer mockStags.Stop()

	// Run the test
	results := runTest(config)

	// Print results
	printResults(results, mockStags)
}

func parseFlags() *TestConfig {
	config := &TestConfig{}

	flag.StringVar(&config.RelayURL, "relay-url", "ws://localhost:8080/ws/streamkit", "Relay WebSocket URL")
	flag.StringVar(&config.StagsURL, "stags-url", "http://localhost:8000/ingest", "Stags ingest URL")
	flag.IntVar(&config.Concurrent, "concurrent", 3, "Number of concurrent connections")
	flag.DurationVar(&config.SendInterval, "interval", 100*time.Millisecond, "Interval between packet sends")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose logging")
	flag.Parse()

	// Default packet files
	config.PacketFiles = []string{
		"testdata/sample_packet_1.json",
		"testdata/sample_packet_2.json",
		"testdata/sample_packet_3.json",
		"testdata/sample_packet_4.json",
		"testdata/sample_packet_5.json",
	}

	return config
}

func startMockStagsServer(port int) *MockStagsServer {
	mock := &MockStagsServer{
		port:      port,
		responses: make([]StagsResponse, 0),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ingest", mock.handleIngest)
	mux.HandleFunc("/responses", mock.handleGetResponses)

	mock.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		log.Printf("Starting mock Stags server on port %d", port)
		if err := mock.server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Mock Stags server error: %v", err)
		}
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	return mock
}

func (m *MockStagsServer) handleIngest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	response := StagsResponse{
		Timestamp:  time.Now(),
		Method:     r.Method,
		Path:       r.URL.Path,
		Headers:    r.Header,
		Body:       string(body),
		StatusCode: http.StatusOK,
	}

	m.mutex.Lock()
	m.responses = append(m.responses, response)
	m.mutex.Unlock()

	// Parse the batch to get event count
	var batch map[string]interface{}
	if err := json.Unmarshal(body, &batch); err == nil {
		if events, ok := batch["events"].([]interface{}); ok {
			log.Printf("Mock Stags: Received batch with %d events", len(events))
		}
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true, "message": "Batch processed successfully"}`))
}

func (m *MockStagsServer) handleGetResponses(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	responses := make([]StagsResponse, len(m.responses))
	copy(responses, m.responses)
	m.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (m *MockStagsServer) Stop() {
	if m.server != nil {
		m.server.Close()
	}
}

func (m *MockStagsServer) GetResponseCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.responses)
}

func runTest(config *TestConfig) *TestResults {
	results := &TestResults{
		StartTime: time.Now(),
	}

	log.Printf("Starting test with %d concurrent connections", config.Concurrent)

	var wg sync.WaitGroup

	// Start concurrent test connections
	for i := 0; i < config.Concurrent; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			runTestClient(clientID, config, results)
		}(i)
	}

	wg.Wait()
	results.EndTime = time.Now()

	return results
}

func runTestClient(clientID int, config *TestConfig, results *TestResults) {
	log.Printf("Client %d: Starting connection to %s", clientID, config.RelayURL)

	// Parse URL and add query parameters
	u, err := url.Parse(config.RelayURL)
	if err != nil {
		results.addError(fmt.Sprintf("Client %d: Invalid URL: %v", clientID, err))
		return
	}

	q := u.Query()
	q.Set("session_id", fmt.Sprintf("test_session_%03d", clientID))
	q.Set("device_id", fmt.Sprintf("test_device_%03d", clientID))
	u.RawQuery = q.Encode()

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		results.addError(fmt.Sprintf("Client %d: Failed to connect: %v", clientID, err))
		return
	}
	defer conn.Close()

	log.Printf("Client %d: Connected successfully", clientID)

	// Send packets
	for i, packetFile := range config.PacketFiles {
		if err := sendPacket(conn, packetFile, clientID, i+1, config.Verbose); err != nil {
			results.addError(fmt.Sprintf("Client %d: Failed to send packet %s: %v", clientID, packetFile, err))
		} else {
			results.incrementPacketsSent()
			log.Printf("Client %d: Sent packet %d from %s", clientID, i+1, packetFile)
		}

		time.Sleep(config.SendInterval)
	}

	log.Printf("Client %d: Finished sending packets", clientID)
}

func sendPacket(conn *websocket.Conn, packetFile string, clientID, packetNum int, verbose bool) error {
	// Read packet data
	data, err := os.ReadFile(packetFile)
	if err != nil {
		return fmt.Errorf("failed to read packet file: %w", err)
	}

	// Parse JSON packet
	var packetData PacketData
	if err := json.Unmarshal(data, &packetData); err != nil {
		return fmt.Errorf("failed to parse packet JSON: %w", err)
	}

	// Modify packet for this client
	packetData.Header.ClientID = fmt.Sprintf("test_client_%03d", clientID)
	packetData.Header.SessionID = fmt.Sprintf("test_session_%03d", clientID)
	packetData.Header.FrameNumber = uint64(packetNum)
	packetData.Header.Timestamp = time.Now().Unix()

	// Convert to binary StreamKit format
	binaryPacket, err := encodePacketToBinary(packetData)
	if err != nil {
		return fmt.Errorf("failed to encode packet: %w", err)
	}

	if verbose {
		log.Printf("Client %d: Sending binary packet of %d bytes", clientID, len(binaryPacket))
	}

	// Send packet
	return conn.WriteMessage(websocket.BinaryMessage, binaryPacket)
}

func encodePacketToBinary(packet PacketData) ([]byte, error) {
	var buf bytes.Buffer

	// Write magic string
	buf.WriteString("STMK")

	// Write version
	binary.Write(&buf, binary.LittleEndian, packet.Header.Version)

	// Prepare header JSON
	headerMeta := map[string]interface{}{
		"timestamp":    packet.Header.Timestamp,
		"frame_number": packet.Header.FrameNumber,
		"session_id":   packet.Header.SessionID,
		"client_id":    packet.Header.ClientID,
		"total_size":   packet.Header.TotalSize,
	}

	headerJSON, err := json.Marshal(headerMeta)
	if err != nil {
		return nil, err
	}

	// Calculate and write header size
	minHeaderSize := 4 + 2 + 4 + 4 // magic + version + header_size + stream_count
	headerSize := uint32(minHeaderSize + len(headerJSON))
	binary.Write(&buf, binary.LittleEndian, headerSize)

	// Write stream count
	binary.Write(&buf, binary.LittleEndian, packet.Header.StreamCount)

	// Write header JSON
	buf.Write(headerJSON)

	// Write streams
	for _, stream := range packet.Streams {
		// Marshal stream metadata
		metadataJSON, err := json.Marshal(stream.Metadata)
		if err != nil {
			return nil, err
		}

		// Marshal stream data
		dataJSON, err := json.Marshal(stream.Data)
		if err != nil {
			return nil, err
		}

		// Compress data based on compression type
		compressedData, err := compressData(dataJSON, stream.Metadata.Compression)
		if err != nil {
			return nil, err
		}

		// Write metadata size
		binary.Write(&buf, binary.LittleEndian, uint32(len(metadataJSON)))

		// Write metadata
		buf.Write(metadataJSON)

		// Write compressed data
		buf.Write(compressedData)
	}

	return buf.Bytes(), nil
}

func compressData(data []byte, compression string) ([]byte, error) {
	switch compression {
	case "none":
		return data, nil
	case "zlib":
		var buf bytes.Buffer
		writer := zlib.NewWriter(&buf)
		writer.Write(data)
		writer.Close()
		return buf.Bytes(), nil
	case "lz4":
		var buf bytes.Buffer
		writer := lz4.NewWriter(&buf)
		writer.Write(data)
		writer.Close()
		return buf.Bytes(), nil
	case "jpeg":
		// For JPEG, just return the data as-is (it's already compressed)
		return data, nil
	default:
		return data, nil
	}
}

func (r *TestResults) addError(err string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Errors = append(r.Errors, err)
}

func (r *TestResults) incrementPacketsSent() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.PacketsSent++
}

func printResults(results *TestResults, mockStags *MockStagsServer) {
	duration := results.EndTime.Sub(results.StartTime)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("TEST RESULTS")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("Test Duration: %v\n", duration)
	fmt.Printf("Packets Sent: %d\n", results.PacketsSent)
	fmt.Printf("Stags Responses: %d\n", mockStags.GetResponseCount())

	if len(results.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(results.Errors))
		for i, err := range results.Errors {
			fmt.Printf("  %d. %s\n", i+1, err)
		}
	} else {
		fmt.Println("Errors: None")
	}

	// Determine pass/fail
	success := len(results.Errors) == 0 && mockStags.GetResponseCount() > 0

	fmt.Println(strings.Repeat("-", 60))
	if success {
		fmt.Println("✅ TEST PASSED")
		fmt.Println("- All packets sent successfully")
		fmt.Println("- Relay received and processed packets")
		fmt.Println("- Mock Stags server received batched events")
	} else {
		fmt.Println("❌ TEST FAILED")
		if len(results.Errors) > 0 {
			fmt.Println("- Errors occurred during packet transmission")
		}
		if mockStags.GetResponseCount() == 0 {
			fmt.Println("- No events received by Stags server")
		}
	}
	fmt.Println(strings.Repeat("=", 60))

	// Show sample Stags response if available
	if mockStags.GetResponseCount() > 0 {
		fmt.Println("\nSample Stags Response:")
		resp, err := http.Get("http://localhost:8000/responses")
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			fmt.Println(string(body))
		}
	}
}

