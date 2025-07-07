# Tabular Relay Service - Focused Implementation Plan

## Architecture Overview

```
StreamKit → [Compressed Binary] → Relay → [Enriched JSON] → Stags HTTP API
```

The Relay is where the **intelligence** lives - it's not just a passthrough, it's where we transform raw spatial data into meaningful events.

## Core Design Principles

1. **Single Binary** - One Go executable, no external dependencies
2. **Stateless** - Can scale horizontally by running more instances
3. **Smart Processing** - Rich transformation logic, not just decompression
4. **Direct to Stags** - HTTP POST to Stags API, no intermediate queues

## Implementation Structure

### 1. WebSocket Server (`pkg/server/`)
```go
type Server struct {
    port       int
    stag       *StagClient
    processor  *Processor
    sessions   sync.Map  // clientID → session metadata
}

// Handle concurrent connections
func (s *Server) HandleConnection(conn *websocket.Conn) {
    clientID := generateClientID()
    session := &Session{
        clientID: clientID,
        conn:     conn,
        started:  time.Now(),
    }
    
    // Process packets in goroutine
    go s.processPackets(session)
}
```

### 2. Packet Processor (`pkg/processor/`)
The brain of the relay - handles all the complex transformations:

```go
type Processor struct {
    meshProcessor   *MeshProcessor
    poseProcessor   *PoseProcessor
    depthProcessor  *DepthProcessor
    // ... other stream processors
}

func (p *Processor) ProcessPacket(raw []byte) (*SpatialEvent, error) {
    // 1. Parse header and validate
    header, offset := parseHeader(raw)
    
    // 2. Extract metadata
    meta, offset := parseMetadata(raw[offset:])
    
    // 3. Process each stream
    streams := make([]ProcessedStream, 0)
    for i := 0; i < meta.StreamCount; i++ {
        stream := p.processStream(raw[offset:])
        streams = append(streams, stream)
        offset += stream.Size
    }
    
    // 4. Build enriched event
    return p.buildSpatialEvent(header, meta, streams)
}
```

### 3. Stream-Specific Processors

#### Mesh Processor
```go
type MeshProcessor struct {
    decimator     *MeshDecimator
    classifier    *MeshClassifier
    deltaEncoder  *DeltaEncoder
}

func (mp *MeshProcessor) Process(data []byte, quality Quality) (*MeshData, error) {
    // 1. Decompress mesh data
    decompressed := zlib.Decompress(data)
    
    // 2. Parse mesh anchors
    anchors := parseMeshAnchors(decompressed)
    
    // 3. Intelligent processing:
    //    - Simplify mesh based on quality settings
    //    - Classify surfaces (floor, wall, ceiling)
    //    - Generate deltas from previous frame
    //    - Remove duplicate vertices
    //    - Optimize face ordering
    
    return &MeshData{
        Anchors:       anchors,
        Vertices:      optimized.Vertices,
        Faces:         optimized.Faces,
        Classifications: classifications,
        Delta:         delta,
    }
}
```

#### Pose Processor
```go
func (pp *PoseProcessor) Process(data []byte) (*PoseData, error) {
    // 1. Decompress
    pose := decompressLZ4(data)
    
    // 2. Coordinate transformation
    //    - Convert from ARKit to Stags coordinate system
    //    - Smooth noisy IMU data
    //    - Calculate velocity/acceleration
    
    // 3. Trajectory analysis
    //    - Detect stationary periods
    //    - Mark significant pose changes
    
    return &PoseData{
        Transform:    normalizedTransform,
        Velocity:     velocity,
        Acceleration: acceleration,
        Quality:      trackingQuality,
    }
}
```

### 4. Enrichment Pipeline (`pkg/enricher/`)
```go
type Enricher struct {
    geoLocator   *GeoLocator
    deviceCache  *DeviceCache
}

func (e *Enricher) Enrich(event *SpatialEvent, session *Session) {
    // Server-side enrichment
    event.ServerTimestamp = time.Now()
    event.ProcessingLatency = time.Since(event.FrameTimestamp)
    
    // Session context
    event.SessionDuration = time.Since(session.Started)
    event.PacketsInSession = session.PacketCount
    
    // Device info (cached)
    if device := e.deviceCache.Get(session.ClientID); device != nil {
        event.DeviceModel = device.Model
        event.OSVersion = device.OSVersion
    }
    
    // Spatial context
    event.SpaceID = e.inferSpaceID(event.Pose)
    event.FloorLevel = e.estimateFloor(event.Pose.Transform.Y)
}
```

### 5. Stag Client (`pkg/stag/`)
```go
type StagClient struct {
    endpoint   string
    httpClient *http.Client
}

func (s *StagClient) UpdateStag(event *SpatialEvent) error {
    // 1. Convert to Stag's expected format
    stagUpdate := &StagUpdate{
        EventID:    event.EventID,
        SpaceID:    event.SpaceID,
        Timestamp:  event.ServerTimestamp,
        Mutations:  s.buildMutations(event),
    }
    
    // 2. POST with retries
    return s.postWithRetry(stagUpdate, 3)
}

func (s *StagClient) buildMutations(event *SpatialEvent) []Mutation {
    mutations := []Mutation{}
    
    // Mesh mutations
    if event.Mesh != nil {
        mutations = append(mutations, Mutation{
            Type: "mesh_update",
            Data: event.Mesh,
        })
    }
    
    // Pose mutations
    if event.Pose != nil {
        mutations = append(mutations, Mutation{
            Type: "trajectory_append",
            Data: event.Pose,
        })
    }
    
    return mutations
}
```

## Data Structures

### SpatialEvent (Relay → Stag)
```go
type SpatialEvent struct {
    // Identity
    EventID         string    `json:"eventId"`
    ClientID        string    `json:"clientId"`
    SessionID       string    `json:"sessionId"`
    SequenceNumber  uint64    `json:"sequenceNumber"`
    
    // Timing
    FrameTimestamp    time.Time `json:"frameTimestamp"`
    ServerTimestamp   time.Time `json:"serverTimestamp"`
    ProcessingLatency Duration  `json:"processingLatencyMs"`
    
    // Spatial data
    Pose  *PoseData  `json:"pose,omitempty"`
    Mesh  *MeshData  `json:"mesh,omitempty"`
    Depth *DepthData `json:"depth,omitempty"`
    RGB   *ImageData `json:"rgb,omitempty"`
    
    // Enrichments
    SpaceID         string  `json:"spaceId"`
    FloorLevel      int     `json:"floorLevel"`
    DeviceModel     string  `json:"deviceModel"`
    SessionDuration int64   `json:"sessionDurationMs"`
    
    // Quality metrics
    MeshQuality     float32 `json:"meshQuality"`
    TrackingQuality string  `json:"trackingQuality"`
}
```

## Implementation Steps

### Day 1: Core Pipeline
1. **WebSocket server** accepting StreamKit connections
2. **Basic packet parser** for SKIT_PKT_V1 format
3. **Decompression** for zlib/lz4
4. **Direct HTTP POST** to a mock Stag endpoint

### Day 2: Smart Processing
1. **Mesh processor** with decimation and classification
2. **Pose processor** with coordinate transformation
3. **Delta encoding** for incremental updates
4. **Enrichment** with server-side metadata

### Day 3: Production Features
1. **Connection management** with graceful shutdown
2. **Error handling** and retry logic
3. **Performance optimization** (parallel processing)
4. **Basic monitoring** (log metrics, not Prometheus)

### Day 4: Testing & Validation
1. **Packet replay tool** for testing
2. **Load testing** with synthetic data
3. **Integration tests** with real StreamKit
4. **Performance benchmarks**

## Why This Architecture Scales

1. **Stateless Design** - Just add more Relay instances behind a load balancer
2. **Efficient Processing** - Goroutines handle concurrent connections
3. **Smart Batching** - Can batch multiple events to Stag in future
4. **Cache Layer Ready** - Can add Redis for device metadata later
5. **Queue Ready** - Can insert Kafka between Relay→Stag later if needed

## Configuration (Simple YAML)
```yaml
server:
  port: 8080
  maxConnections: 1000

stag:
  endpoint: "http://stag-ingest.tabular.local"
  timeout: 5s
  retries: 3

processing:
  meshQuality: "balanced"
  enableDeltaEncoding: true
  coordinateSystem: "stag"  # or "arkit"

logging:
  level: "info"
  format: "json"
```

## Next Immediate Steps

1. **Set up Go module**:
   ```bash
   cd relays
   go mod init github.com/tabular/relay
   go get github.com/gorilla/websocket
   ```

2. **Create main.go** with WebSocket server skeleton

3. **Implement packet parser** for StreamKit format

4. **Add first processor** (start with Pose, it's simpler)

5. **Mock Stag client** that logs JSON to verify output

This gives you a production-quality foundation that's simple today but ready to scale tomorrow. The intelligence is in the transformations, not in the infrastructure complexity. 