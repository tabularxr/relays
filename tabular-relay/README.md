# Tabular Relay

A high-performance, production-ready Go service that processes real-time spatial data streams from StreamKit clients and forwards enriched events to the Stags spatial processing system.

## Overview

The Tabular Relay serves as the **intelligent processing bridge** between AR/VR devices running StreamKit and the Stags spatial data system. It transforms raw binary spatial data streams into enriched, structured events optimized for downstream processing.

### What It Does

- **Receives**: Binary StreamKit packets over WebSocket connections
- **Processes**: Decompresses, parses, and validates spatial data streams
- **Transforms**: Applies intelligent processing (mesh optimization, pose smoothing, coordinate transforms)
- **Enriches**: Adds server-side metadata and contextual information
- **Delivers**: Batches and forwards processed events to Stags via HTTP API

### Architecture

```
StreamKit Clients → WebSocket → Relay → HTTP/JSON → Stags
  (iOS/Android)       (Binary)    (Go)    (Events)   (Spatial DB)
```

## Key Features

### 🚀 High Performance
- **Multi-threaded processing** with configurable worker pools
- **Concurrent connection handling** supporting hundreds of simultaneous clients
- **Sub-10ms processing latency** for real-time responsiveness
- **Memory efficient** with minimal per-connection overhead

### 🧠 Intelligent Processing
- **Stream-aware processing** for different data types (mesh, pose, depth, camera)
- **Compression support** (zlib, LZ4, JPEG passthrough)
- **Delta encoding** for incremental mesh updates
- **Pose smoothing** and coordinate system transformations
- **Mesh optimization** with decimation and surface classification

### 🔄 Robust Data Handling
- **Intelligent batching** with configurable time/size thresholds
- **Deduplication** and change detection to minimize redundant data
- **Retry logic** with exponential backoff for downstream failures
- **Graceful degradation** under high load conditions

### 🛡️ Production Ready
- **Comprehensive logging** with structured JSON output
- **Health monitoring** and statistics collection
- **Graceful shutdown** with proper resource cleanup
- **Docker support** for containerized deployment
- **Configuration flexibility** via environment variables, files, or flags

## Quick Start

### Prerequisites

- Go 1.21 or later
- Docker (optional, for containerized deployment)

### Installation & Setup

1. **Clone and build:**
   ```bash
   cd tabular-relay
   go mod tidy
   go build -o relay cmd/relay/main.go
   ```

2. **Run with default configuration:**
   ```bash
   ./relay
   ```

3. **Run with custom configuration:**
   ```bash
   export RELAY_PORT=9090
   export RELAY_STAGS_ENDPOINT=https://your-stags-instance.com/ingest
   export RELAY_LOG_LEVEL=debug
   ./relay
   ```

### Docker Deployment

1. **Build Docker image:**
   ```bash
   docker build -t tabular-relay .
   ```

2. **Run container:**
   ```bash
   docker run -p 8080:8080 \
     -e RELAY_STAGS_ENDPOINT=https://your-stags.com/ingest \
     -e RELAY_LOG_LEVEL=info \
     tabular-relay
   ```

## Configuration

### Configuration Methods

Configuration is loaded in this priority order:
1. **Command-line flags** (highest priority)
2. **Environment variables** (prefixed with `RELAY_`)
3. **Configuration file** (`relay.yaml`)
4. **Default values** (lowest priority)

### Core Configuration Options

| Setting | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| **Server Settings** | | | |
| Port | `RELAY_PORT` | `8080` | WebSocket listener port |
| Max Clients | `RELAY_MAX_CLIENTS` | `100` | Maximum concurrent connections |
| Worker Threads | `RELAY_WORKER_THREADS` | `4` | Processing worker count |
| **Processing Settings** | | | |
| Batch Interval | `RELAY_BATCH_INTERVAL_MS` | `100` | Event batching interval (ms) |
| Snapshot Threshold | `RELAY_SNAPSHOT_THRESHOLD` | `0.1` | Change detection threshold (0.0-1.0) |
| **Output Settings** | | | |
| Stags Endpoint | `RELAY_STAGS_ENDPOINT` | `http://localhost:8000/ingest` | Target Stags API endpoint |
| Retry Attempts | `RELAY_RETRY_ATTEMPTS` | `3` | HTTP retry attempts |
| Retry Delay | `RELAY_RETRY_DELAY` | `2s` | Retry delay duration |
| **Logging Settings** | | | |
| Log Level | `RELAY_LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |

### Advanced Configuration

```yaml
# relay.yaml
server:
  port: 8080
  max_clients: 500
  worker_threads: 8
  read_timeout: 60s
  write_timeout: 10s
  ping_interval: 30s

processing:
  batch_interval_ms: 50
  snapshot_threshold: 0.05
  enable_mesh_optimization: true
  enable_pose_smoothing: true
  coordinate_system: "stags"

output:
  stags_endpoint: "https://production-stags.company.com/ingest"
  timeout: 10s
  retry_attempts: 5
  retry_delay: 1s
  max_batch_size: 100

logging:
  level: "info"
  format: "json"
  enable_caller: true
```

## StreamKit Protocol

### Binary Packet Format

The relay expects StreamKit packets in this binary format:

```
[4 bytes] Magic string "STMK"
[2 bytes] Version (little-endian)
[4 bytes] Header size (little-endian)
[4 bytes] Stream count (little-endian)
[N bytes] Header JSON metadata
[Stream 1 data...]
[Stream 2 data...]
...
```

### Stream Data Format

Each stream contains:
```
[4 bytes] Metadata size (little-endian)
[N bytes] Metadata JSON
[M bytes] Compressed stream data
```

### Supported Stream Types

| Stream Type | Description | Compression | Output |
|-------------|-------------|-------------|---------|
| **mesh** | 3D geometry with anchor transforms | zlib/lz4 | Optimized mesh events |
| **pose** | Device pose and IMU data | lz4 | Smoothed pose events |
| **camera** | RGB imagery with intrinsics | jpeg | Camera frame events |
| **depth** | LiDAR depth maps with confidence | zlib | Depth map events |
| **pointCloud** | 3D point clouds with colors | lz4 | Point cloud events |
| **lighting** | Environmental lighting estimation | none | Lighting events |

### Sample Connection

```javascript
// JavaScript client example
const ws = new WebSocket('ws://localhost:8080/ws/streamkit?session_id=sess_123&device_id=device_456');

ws.onopen = () => {
    console.log('Connected to relay');
    // Send binary StreamKit packet
    ws.send(binaryPacketData);
};

ws.onmessage = (event) => {
    console.log('Received:', event.data);
};
```

## API Endpoints

### WebSocket Endpoints

#### `GET /ws/streamkit`
Primary endpoint for StreamKit client connections.

**Query Parameters:**
- `session_id` (required): Unique session identifier
- `device_id` (required): Device identifier
- `app_version` (optional): Client application version

**Connection Flow:**
1. Client connects with query parameters
2. Relay validates and registers connection
3. Client sends binary StreamKit packets
4. Relay processes and forwards to Stags
5. Connection maintained until client disconnects

### HTTP Endpoints

#### `GET /health`
Health check endpoint returning system status and statistics.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2023-12-20T10:30:00Z",
  "uptime": "2h45m30s",
  "connections": {
    "active": 25,
    "total": 150
  },
  "processing": {
    "events_processed": 12500,
    "events_per_second": 45.2,
    "avg_processing_time_ms": 8.5
  },
  "output": {
    "batches_sent": 125,
    "success_rate": 0.996,
    "last_successful_send": "2023-12-20T10:29:45Z"
  }
}
```

## Testing

### Unit Tests

Run the complete test suite:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Run specific component tests:
```bash
go test ./relay/parser/
go test ./relay/transformer/
go test ./relay/updater/
```

### Integration Testing

The project includes a comprehensive integration test that validates end-to-end functionality:

#### Automated Test Suite

```bash
# Run the complete integration test
./run_test.sh

# Or run manually
go run test_client.go
```

#### Custom Test Parameters

```bash
go run test_client.go \
  -relay-url ws://localhost:8080/ws/streamkit \
  -concurrent 10 \
  -packets-per-client 20 \
  -interval 25ms \
  -verbose
```

#### Test Features

- **Mock Stags Server**: Automatically spins up a mock HTTP server
- **Concurrent Testing**: Tests multiple simultaneous connections
- **Packet Variety**: Uses 5 different sample packet types
- **Performance Metrics**: Reports latency, throughput, and success rates
- **Failure Detection**: Validates HTTP responses and error handling

### Test Data

Sample StreamKit packets in `testdata/`:

- `sample_packet_1.json`: Mesh + Pose data (basic AR scene)
- `sample_packet_2.json`: Camera + Depth + Lighting (rich capture)
- `sample_packet_3.json`: Multi-anchor mesh + Point cloud (complex scene)
- `sample_packet_4.json`: Pose with IMU data (motion tracking)
- `sample_packet_5.json`: High-resolution camera + Depth (detailed capture)

### Load Testing

For performance testing under load:

```bash
# Test with high concurrency
go run test_client.go -concurrent 100 -interval 10ms

# Test with sustained load
go run test_client.go -concurrent 50 -packets-per-client 1000 -interval 20ms
```

## Deployment

### Container Deployment

The included `Dockerfile` creates an optimized production image:

```dockerfile
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
        - name: RELAY_WORKER_THREADS
          value: "8"
        resources:
          requests:
            memory: "128Mi"
            cpu: "200m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 15
          periodSeconds: 20

---
apiVersion: v1
kind: Service
metadata:
  name: tabular-relay-service
spec:
  selector:
    app: tabular-relay
  ports:
  - protocol: TCP
    port: 8080
    targetPort: 8080
  type: LoadBalancer
```

### Environment-Specific Configurations

#### Development
```bash
export RELAY_LOG_LEVEL=debug
export RELAY_STAGS_ENDPOINT=http://localhost:8000/ingest
export RELAY_WORKER_THREADS=2
```

#### Staging
```bash
export RELAY_LOG_LEVEL=info
export RELAY_STAGS_ENDPOINT=https://staging-stags.company.com/ingest
export RELAY_WORKER_THREADS=4
export RELAY_MAX_CLIENTS=200
```

#### Production
```bash
export RELAY_LOG_LEVEL=warn
export RELAY_STAGS_ENDPOINT=https://production-stags.company.com/ingest
export RELAY_WORKER_THREADS=8
export RELAY_MAX_CLIENTS=1000
export RELAY_BATCH_INTERVAL_MS=50
```

## Monitoring & Observability

### Structured Logging

All log entries use structured JSON format:

```json
{
  "timestamp": "2023-12-20T10:30:00Z",
  "level": "info",
  "component": "transformer",
  "message": "Event processed successfully",
  "session_id": "sess_abc123",
  "device_id": "device_456",
  "event_type": "mesh_update",
  "processing_time_ms": 12.5,
  "streams_processed": 3,
  "output_events": 1
}
```

### Key Metrics

The relay automatically logs these metrics every 30 seconds:

#### Connection Metrics
- `active_connections`: Current WebSocket connections
- `total_connections`: Total connections since startup
- `bytes_received`: Total bytes received from clients
- `bytes_sent`: Total bytes sent to clients

#### Processing Metrics
- `events_processed`: Total events processed
- `events_per_second`: Current processing rate
- `avg_processing_time_ms`: Average processing latency
- `events_dropped`: Events dropped due to errors

#### Output Metrics
- `batches_sent`: Total batches sent to Stags
- `batches_successful`: Successful batch deliveries
- `batches_failed`: Failed batch deliveries
- `total_retries`: Total retry attempts

### Performance Monitoring

#### Typical Performance Characteristics

- **Throughput**: 1,000+ events/second per worker thread
- **Latency**: <10ms processing time per event
- **Memory**: ~50MB baseline + ~1KB per active connection
- **CPU**: <20% on modern hardware under normal load

#### Performance Tuning

1. **For Higher Throughput**:
   ```bash
   export RELAY_WORKER_THREADS=16
   export RELAY_BATCH_INTERVAL_MS=25
   ```

2. **For Lower Latency**:
   ```bash
   export RELAY_BATCH_INTERVAL_MS=10
   export RELAY_WORKER_THREADS=8
   ```

3. **For Memory Optimization**:
   ```bash
   export RELAY_MAX_CLIENTS=50
   export RELAY_BATCH_INTERVAL_MS=200
   ```

## Troubleshooting

### Common Issues

#### Connection Issues

**Problem**: Clients can't connect to relay
```
Error: WebSocket connection failed
```

**Solutions**:
1. Check if relay is running: `curl http://localhost:8080/health`
2. Verify port configuration: `netstat -tlnp | grep 8080`
3. Check firewall settings
4. Validate WebSocket URL format

#### Processing Issues

**Problem**: High processing latency
```json
{"level":"warn","message":"High processing latency detected","latency_ms":150}
```

**Solutions**:
1. Increase worker threads: `export RELAY_WORKER_THREADS=8`
2. Check CPU usage: `top -p $(pgrep relay)`
3. Monitor memory usage: `free -h`
4. Reduce batch interval: `export RELAY_BATCH_INTERVAL_MS=50`

#### Output Issues

**Problem**: Failed to send data to Stags
```json
{"level":"error","message":"Failed to send batch to Stags","error":"connection refused"}
```

**Solutions**:
1. Verify Stags endpoint: `curl -X POST $RELAY_STAGS_ENDPOINT`
2. Check network connectivity
3. Validate API authentication
4. Review Stags server logs

### Debug Mode

Enable comprehensive debugging:

```bash
export RELAY_LOG_LEVEL=debug
./relay
```

Debug logs include:
- Detailed packet parsing information
- Stream processing steps
- Transformation operations
- Batch creation and delivery
- Connection lifecycle events

### Performance Profiling

Enable Go profiling for performance analysis:

```bash
go build -o relay cmd/relay/main.go
RELAY_ENABLE_PPROF=true ./relay
```

Then access profiling endpoints:
- `http://localhost:6060/debug/pprof/` - Profile index
- `http://localhost:6060/debug/pprof/goroutine` - Goroutine dump
- `http://localhost:6060/debug/pprof/heap` - Memory heap
- `http://localhost:6060/debug/pprof/profile` - CPU profile

## Development

### Project Structure

```
tabular-relay/
├── cmd/relay/main.go              # Application entry point
├── relay/
│   ├── config/config.go           # Configuration management
│   ├── gate/
│   │   ├── listener/listener.go   # WebSocket listener
│   │   └── manager/manager.go     # Connection management
│   ├── parser/parser.go           # StreamKit packet parsing
│   ├── transformer/transformer.go # Event transformation
│   ├── updater/updater.go         # Stags output handling
│   └── logging/logging.go         # Structured logging
├── testdata/                      # Test data files
├── test_client.go                 # Integration test client
├── run_test.sh                    # Test automation script
├── Dockerfile                     # Container configuration
├── go.mod                         # Go module dependencies
└── README.md                      # This documentation
```

### Adding New Features

1. **Stream Type Support**:
   - Add parsing logic in `parser/parser.go`
   - Implement transformation in `transformer/transformer.go`
   - Add test data in `testdata/`

2. **Processing Enhancements**:
   - Extend transformer with new algorithms
   - Add configuration options in `config/config.go`
   - Update tests and documentation

3. **Output Formats**:
   - Extend updater with new output targets
   - Add configuration for new endpoints
   - Implement retry and error handling

### Code Quality

- **Linting**: Use `golangci-lint` for code quality
- **Testing**: Maintain >80% test coverage
- **Documentation**: Update README and inline docs
- **Performance**: Benchmark critical paths

### Contributing

1. Fork the repository
2. Create a feature branch
3. Add comprehensive tests
4. Update documentation
5. Ensure all tests pass
6. Submit a pull request

## Security Considerations

### Production Security

1. **Network Security**:
   - Use TLS for WebSocket connections (`wss://`)
   - Implement proper firewall rules
   - Use VPN for internal communication

2. **Authentication**:
   - Validate session and device IDs
   - Implement rate limiting per client
   - Use API keys for Stags communication

3. **Data Protection**:
   - Never log sensitive spatial data
   - Implement proper error handling
   - Use secure session ID generation

4. **Monitoring**:
   - Monitor for unusual traffic patterns
   - Set up alerts for failed authentications
   - Track processing anomalies

## License

[Add your license information here]

---

## Support

For issues, questions, or contributions:
- Review the troubleshooting section above
- Check the integration tests for usage examples
- Consult the parent directory's `RELAY_PLAN.md` for architecture details
- Contact the Tabular development team