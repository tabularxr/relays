# Relays

Intelligent data transformation & transmission services for real-time spatial data processing within the Tabular ecosystem.

## Overview

Relays are **intelligent processing intermediaries** that transform raw spatial data streams into meaningful, enriched events. They serve as the critical bridge between data producers (like AR/VR devices running StreamKit) and data consumers (Stags).

Unlike simple proxy services, relays contain sophisticated transformation logic that:
- Decompresses and parses binary data streams
- Applies intelligent filtering and optimization
- Enriches data with contextual metadata
- Transforms coordinate systems and data formats
- Batches and optimizes data for downstream systems

## Architecture Philosophy

```
Raw Data Producer → [Relay: Intelligence Layer] → Processed Data Consumer
     StreamKit    →    [Spatial Processing]    →        Stags
```

### Core Design Principles

1. **Intelligence, Not Passthrough**: Relays contain the smart processing logic that transforms raw data into meaningful events
2. **Stateless by Design**: Can scale horizontally by running multiple instances
3. **Stream-Aware Processing**: Understands different data stream types (mesh, pose, depth, etc.)
4. **Real-time Optimization**: Applies compression, deduplication, and delta encoding
5. **Contextual Enrichment**: Adds server-side metadata and spatial context

## Relay Types

### Current Implementations

#### tabular-relay
**Production-ready Go implementation** for StreamKit → Stags pipeline
- **Purpose**: Processes AR/VR spatial data streams from iOS/Android devices
- **Input**: Binary StreamKit packets over WebSocket
- **Output**: Enriched JSON events to Stags HTTP API
- **Features**: Multi-threaded processing, intelligent batching, retry logic
- **Status**: ✅ Ready for production deployment

### Future Relay Implementations

#### audio-relay
**Planned**: Audio stream processing for spatial audio applications
- **Purpose**: Process spatial audio data with 3D positioning
- **Input**: Compressed audio streams with positional metadata
- **Output**: Processed audio events with spatial context

#### sensor-relay
**Planned**: IoT sensor data aggregation and processing
- **Purpose**: Aggregate and process environmental sensor data
- **Input**: Multiple sensor streams (temperature, humidity, motion, etc.)
- **Output**: Aggregated environmental events

#### ml-relay
**Planned**: Machine learning inference relay
- **Purpose**: Apply ML models to spatial data streams
- **Input**: Processed spatial data
- **Output**: ML-enriched events (object detection, scene classification, etc.)

## When to Use Relays

### ✅ Use a Relay When:
- You need to transform data formats between systems
- Processing requires stream-specific intelligence (mesh optimization, pose smoothing)
- You need to enrich data with server-side context
- Batching and optimization are required for performance
- You need to bridge different protocols (WebSocket → HTTP, binary → JSON)
- Real-time processing with low latency is critical

### ❌ Don't Use a Relay When:
- Simple proxy/forwarding is sufficient
- No data transformation is needed
- Processing can be done client-side or in the final consumer
- Latency requirements are not strict

## Implementation Patterns

### Common Relay Architecture

```go
// Standard relay components
type Relay struct {
    listener    *Listener     // Protocol-specific input handler
    parser      *Parser       // Input data parsing and validation
    transformer *Transformer  // Core processing logic
    updater     *Updater      // Output batching and delivery
    manager     *Manager      // Connection/session management
}
```

### Processing Pipeline

```
1. Listen: Accept incoming connections/data
2. Parse: Decode and validate input data
3. Transform: Apply intelligent processing
4. Batch: Group and optimize for output
5. Update: Deliver to downstream systems
```

### Key Interfaces

```go
// Input parser interface
type Parser interface {
    Parse([]byte) (*Packet, error)
    ValidatePacket(*Packet) error
}

// Data transformer interface
type Transformer interface {
    Transform(*Packet) ([]*Event, error)
}

// Output updater interface
type Updater interface {
    ProcessEvent(*Event)
    Flush() error
}
```

## Configuration Standards

All relays follow consistent configuration patterns:

```yaml
# Server configuration
server:
  port: 8080
  max_connections: 1000
  worker_threads: 4

# Processing configuration
processing:
  batch_interval_ms: 100
  snapshot_threshold: 0.1
  coordinate_system: "target_system"

# Output configuration
output:
  endpoint: "http://target-system.local/ingest"
  timeout: 5s
  retry_attempts: 3
  retry_delay: 2s

# Logging configuration
logging:
  level: "info"
  format: "json"
```

## Performance Characteristics

### Typical Relay Performance

- **Throughput**: 1000+ events/second per worker thread
- **Latency**: <10ms processing time per event
- **Memory**: ~50MB baseline + ~1KB per active connection
- **CPU**: <20% on modern hardware under normal load

### Scaling Strategies

1. **Horizontal Scaling**: Deploy multiple relay instances behind load balancer
2. **Worker Scaling**: Increase worker threads for CPU-bound processing
3. **Batch Optimization**: Tune batch intervals for throughput vs latency
4. **Connection Pooling**: Optimize downstream HTTP connections

## Development Guidelines

### Creating a New Relay

1. **Define the Data Flow**:
   ```
   Source → [Your Relay] → Destination
   ```

2. **Implement Core Components**:
   - Input listener (WebSocket, HTTP, TCP, etc.)
   - Data parser for your input format
   - Transformer with your processing logic
   - Output updater for your destination

3. **Follow Standards**:
   - Use structured logging (JSON format)
   - Implement graceful shutdown
   - Add comprehensive configuration options
   - Include health checks and metrics

4. **Testing Strategy**:
   - Unit tests for parser and transformer
   - Integration tests with mock endpoints
   - Load tests for performance validation
   - End-to-end tests with real data

### Code Organization

```
your-relay/
├── cmd/relay/main.go              # Application entry point
├── relay/
│   ├── config/config.go           # Configuration management
│   ├── gate/
│   │   ├── listener/listener.go   # Input listener
│   │   └── manager/manager.go     # Connection management
│   ├── parser/parser.go           # Data parsing
│   ├── transformer/transformer.go # Processing logic
│   ├── updater/updater.go         # Output handling
│   └── logging/logging.go         # Logging utilities
├── testdata/                      # Test data files
├── Dockerfile                     # Container configuration
├── go.mod                         # Dependencies
└── README.md                      # Documentation
```

## Monitoring and Observability

### Standard Metrics

All relays should provide:
- **Connection metrics**: Active connections, total connections, connection duration
- **Processing metrics**: Events processed, processing time, error rates
- **Output metrics**: Batches sent, success rates, retry counts
- **System metrics**: Memory usage, CPU usage, goroutine count

### Logging Standards

```json
{
  "timestamp": "2023-12-20T10:30:00Z",
  "level": "info",
  "component": "transformer",
  "message": "Event processed successfully",
  "session_id": "sess_abc123",
  "event_id": "evt_456789",
  "processing_time_ms": 12.5,
  "event_type": "mesh_update"
}
```

## Deployment Patterns

### Container Deployment

```dockerfile
# Multi-stage build for minimal production image
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download && go build -o relay cmd/relay/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/relay .
EXPOSE 8080
CMD ["./relay"]
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tabular-relay
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tabular-relay
  template:
    metadata:
      labels:
        app: tabular-relay
    spec:
      containers:
      - name: relay
        image: tabular-relay:latest
        ports:
        - containerPort: 8080
        env:
        - name: RELAY_STAGS_ENDPOINT
          value: "http://stags-service:8000/ingest"
        - name: RELAY_LOG_LEVEL
          value: "info"
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

## Security Considerations

### Production Security

1. **Network Security**:
   - Use TLS for all external connections
   - Implement proper firewall rules
   - Consider VPN for internal communication

2. **Authentication**:
   - Implement client authentication for WebSocket connections
   - Use API keys or tokens for downstream services
   - Rotate credentials regularly

3. **Data Protection**:
   - Never log sensitive data
   - Implement proper error handling to avoid data leaks
   - Use secure random generation for session IDs

4. **Rate Limiting**:
   - Implement per-client rate limiting
   - Set maximum connection limits
   - Monitor for abuse patterns

## Contributing

### Development Setup

1. **Prerequisites**:
   - Go 1.21 or later
   - Docker (for containerized testing)
   - Make (for build automation)

2. **Getting Started**:
   ```bash
   git clone https://github.com/tabular/relays
   cd relays
   cd tabular-relay  # or your specific relay
   go mod tidy
   go test ./...
   ```

3. **Development Workflow**:
   - Create feature branches from `main`
   - Add comprehensive tests for new functionality
   - Update documentation
   - Ensure all tests pass
   - Submit pull requests

### Code Standards

- **Go**: Follow standard Go conventions and use `gofmt`
- **Testing**: Maintain >80% test coverage
- **Documentation**: Update README and inline docs
- **Performance**: Benchmark critical paths
- **Security**: Never commit secrets or credentials

## License

[Add your license information here]

---

## Next Steps

1. **Explore tabular-relay**: Start with the production-ready implementation
2. **Review RELAY_PLAN.md**: Understand the detailed architecture design
3. **Run Tests**: Validate your environment with the test suite
4. **Deploy**: Follow the deployment guide for your target environment
5. **Monitor**: Set up observability for your relay instances

For questions or support, please see the individual relay documentation or contact the Tabular team.
