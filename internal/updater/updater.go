package updater

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/tabular/relay/pkg/types"
)

// Updater handles batching, diffing, and forwarding to STAG
type Updater struct {
	stagURL     string
	httpClient  *http.Client
	
	// Batching
	batchSize    int
	batchTimeout time.Duration
	eventQueue   []types.SpatialEvent
	queueMutex   sync.Mutex
	
	// Diffing state
	lastMeshes   map[string][]byte // anchorID -> last mesh vertices
	meshMutex    sync.RWMutex
	
	// Compression state (Draco encoder not available in this library)
	compressionEnabled bool
	
	// Control
	stopC        chan struct{}
	wg           sync.WaitGroup
}

// New creates a new Updater instance
func New(stagURL string, batchSize int, batchTimeout time.Duration) *Updater {
	return &Updater{
		stagURL:            stagURL,
		httpClient:         &http.Client{Timeout: 10 * time.Second},
		batchSize:          batchSize,
		batchTimeout:       batchTimeout,
		eventQueue:         make([]types.SpatialEvent, 0, batchSize),
		lastMeshes:         make(map[string][]byte),
		compressionEnabled: true, // Enable simple compression
		stopC:              make(chan struct{}),
	}
}

// Start begins the updater operations
func (u *Updater) Start() {
	u.wg.Add(1)
	go u.batchProcessor()
}

// Stop gracefully shuts down the updater
func (u *Updater) Stop() {
	close(u.stopC)
	u.wg.Wait()
}

// ProcessEvent adds an event to the processing queue
func (u *Updater) ProcessEvent(event types.SpatialEvent) error {
	// Apply diffing to meshes
	processedEvent := u.applyMeshDiffing(event)
	
	u.queueMutex.Lock()
	defer u.queueMutex.Unlock()
	
	u.eventQueue = append(u.eventQueue, processedEvent)
	
	// Trigger immediate batch if queue is full
	if len(u.eventQueue) >= u.batchSize {
		go u.flushBatch()
	}
	
	return nil
}

// applyMeshDiffing converts full meshes to diffs when possible
func (u *Updater) applyMeshDiffing(event types.SpatialEvent) types.SpatialEvent {
	if len(event.Meshes) == 0 {
		return event
	}
	
	u.meshMutex.Lock()
	defer u.meshMutex.Unlock()
	
	processedMeshes := make([]types.MeshDiff, 0, len(event.Meshes))
	
	for _, mesh := range event.Meshes {
		if mesh.IsDelta {
			// Already a delta, keep as-is
			processedMeshes = append(processedMeshes, mesh)
			continue
		}
		
		// Check if we have a previous version
		lastVertices, exists := u.lastMeshes[mesh.AnchorID]
		if !exists || len(lastVertices) == 0 {
			// First mesh for this anchor, store as full mesh
			u.lastMeshes[mesh.AnchorID] = mesh.VerticesDelta
			processedMeshes = append(processedMeshes, mesh)
			continue
		}
		
		// Calculate similarity
		similarity := u.calculateVertexSimilarity(lastVertices, mesh.VerticesDelta)
		
		if similarity > 0.8 { // More than 80% similar
			// Create delta
			delta := u.createVertexDelta(lastVertices, mesh.VerticesDelta)
			if len(delta) < int(float64(len(mesh.VerticesDelta))*0.7) { // Delta is smaller
				processedMesh := types.MeshDiff{
					AnchorID:      mesh.AnchorID,
					VerticesDelta: delta,
					FacesDelta:    mesh.FacesDelta, // Faces deltas are more complex, defer for MVP
					IsDelta:       true,
				}
				processedMeshes = append(processedMeshes, processedMesh)
				
				// Update stored mesh
				u.lastMeshes[mesh.AnchorID] = mesh.VerticesDelta
				continue
			}
		}
		
		// Send as full mesh if delta isn't beneficial
		u.lastMeshes[mesh.AnchorID] = mesh.VerticesDelta
		processedMeshes = append(processedMeshes, mesh)
	}
	
	// Create new event with processed meshes
	processedEvent := event
	processedEvent.Meshes = processedMeshes
	return processedEvent
}

// calculateVertexSimilarity computes similarity between two vertex buffers
func (u *Updater) calculateVertexSimilarity(a, b []byte) float64 {
	if len(a) != len(b) {
		return 0.0
	}
	
	if len(a) == 0 {
		return 1.0
	}
	
	// Simple byte-wise comparison for MVP
	// In production, should compare as float32 vertices with spatial tolerance
	matches := 0
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			matches++
		}
	}
	
	return float64(matches) / float64(len(a))
}

// createVertexDelta creates a simple delta between vertex buffers
func (u *Updater) createVertexDelta(old, new []byte) []byte {
	// Simple XOR delta for MVP
	// In production, should use proper vertex diffing algorithms
	if len(old) != len(new) {
		return new // Return full mesh if sizes don't match
	}
	
	delta := make([]byte, len(new))
	for i := 0; i < len(new); i++ {
		delta[i] = old[i] ^ new[i]
	}
	
	return delta
}

// batchProcessor handles periodic batch flushing
func (u *Updater) batchProcessor() {
	defer u.wg.Done()
	
	ticker := time.NewTicker(u.batchTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			u.flushBatch()
		case <-u.stopC:
			// Final flush before stopping
			u.flushBatch()
			return
		}
	}
}

// flushBatch sends the current batch to STAG
func (u *Updater) flushBatch() {
	u.queueMutex.Lock()
	if len(u.eventQueue) == 0 {
		u.queueMutex.Unlock()
		return
	}
	
	// Copy events and clear queue
	events := make([]types.SpatialEvent, len(u.eventQueue))
	copy(events, u.eventQueue)
	u.eventQueue = u.eventQueue[:0]
	u.queueMutex.Unlock()
	
	// Send to STAG
	if err := u.sendToSTAG(events); err != nil {
		log.Printf("Failed to send batch to STAG: %v", err)
		// TODO: Implement retry logic or dead letter queue
	}
}

// sendToSTAG sends events to the STAG service
func (u *Updater) sendToSTAG(events []types.SpatialEvent) error {
	if len(events) == 0 {
		return nil
	}
	
	// Compress mesh data in events before sending
	compressedEvents := make([]types.SpatialEvent, len(events))
	copy(compressedEvents, events)
	
	for i := range compressedEvents {
		for j := range compressedEvents[i].Meshes {
			mesh := &compressedEvents[i].Meshes[j]
			
			// Compress vertices if present
			if len(mesh.VerticesDelta) > 0 {
				compressed, bytesSaved, err := u.compressMeshData(mesh.VerticesDelta)
				if err != nil {
					log.Printf("Failed to compress mesh vertices: %v", err)
					// Continue with uncompressed data
				} else {
					mesh.VerticesDelta = compressed
					// Note: metrics recording would need to be passed in via dependency injection
					// For now, we log the savings
					if bytesSaved > 0 {
						log.Printf("Compression saved %d bytes", bytesSaved)
					}
				}
			}
			
			// Compress faces if present (similar process)
			if len(mesh.FacesDelta) > 0 {
				// For MVP, faces are kept as-is since they're typically indices
				// In production, faces could also be compressed or encoded differently
			}
		}
	}
	
	// Create batch payload
	batch := map[string]interface{}{
		"events":    compressedEvents,
		"timestamp": time.Now().UnixMilli(),
		"count":     len(compressedEvents),
	}
	
	// Marshal to JSON
	payload, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}
	
	// Create request
	req, err := http.NewRequestWithContext(
		context.Background(),
		"POST",
		u.stagURL+"/ingest",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// Send request
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("STAG returned status %d", resp.StatusCode)
	}
	
	log.Printf("Successfully sent batch of %d events to STAG", len(events))
	return nil
}

// compressMeshData compresses vertex data using simple compression
// Note: Draco encoder not available in qmuntal/draco-go (decode-only library)
// Implementing simple gzip compression for MVP
func (u *Updater) compressMeshData(vertices []byte) ([]byte, int, error) {
	if len(vertices) == 0 || !u.compressionEnabled {
		return vertices, 0, nil
	}

	startTime := time.Now()
	originalSize := len(vertices)

	// Use simple gzip compression as fallback
	var compressed bytes.Buffer
	gzWriter := gzip.NewWriter(&compressed)
	
	_, err := gzWriter.Write(vertices)
	if err != nil {
		gzWriter.Close()
		return nil, 0, fmt.Errorf("gzip compression failed: %w", err)
	}
	
	err = gzWriter.Close()
	if err != nil {
		return nil, 0, fmt.Errorf("gzip close failed: %w", err)
	}

	compressionTime := time.Since(startTime).Seconds()
	compressedData := compressed.Bytes()
	compressedSize := len(compressedData)
	compressionRatio := float64(compressedSize) / float64(originalSize)
	bytesSaved := originalSize - compressedSize
	
	log.Printf("Compressed mesh (gzip): %d -> %d bytes (%.1f%% ratio, %d bytes saved, %.2fms)", 
		originalSize, compressedSize, compressionRatio*100, bytesSaved, compressionTime*1000)
	
	return compressedData, bytesSaved, nil
}

// GetStats returns updater statistics
func (u *Updater) GetStats() map[string]interface{} {
	u.queueMutex.Lock()
	queueLength := len(u.eventQueue)
	u.queueMutex.Unlock()
	
	u.meshMutex.RLock()
	trackedMeshes := len(u.lastMeshes)
	u.meshMutex.RUnlock()
	
	return map[string]interface{}{
		"queue_length":   queueLength,
		"tracked_meshes": trackedMeshes,
		"batch_size":     u.batchSize,
		"batch_timeout":  u.batchTimeout.String(),
	}
}

// ClearMeshHistory removes old mesh data to free memory
func (u *Updater) ClearMeshHistory(anchorID string) {
	u.meshMutex.Lock()
	defer u.meshMutex.Unlock()
	delete(u.lastMeshes, anchorID)
}