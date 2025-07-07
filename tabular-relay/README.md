# Tabular Relay

A minimal, configurable relay for Tabular's StreamKit → Stags pipeline, implemented in Go.

## Overview

The Tabular Relay acts as an intermediary between StreamKit clients (iOS/AR devices) and the Stags spatial data processing system. It receives real-time spatial data streams via WebSocket, processes and enriches them, then batches and forwards the data to Stags for ingestion.

## Architecture

```
StreamKit Client → WebSocket → Relay → HTTP/JSON → Stags
```

### Components

- **gate.listener**: WebSocket listener for StreamKit clients
- **gate.manager**: Connection management and health monitoring
- **parser**: StreamKit packet parsing and decompression
- **transformer**: Event enrichment and normalization
- **updater**: Batching, deduplication, and Stags API integration
- **config**: Configuration management
- **logging**: Structured logging wrapper

## Features

- **High-throughput WebSocket handling** with configurable connection limits
- **Multi-threaded processing** with worker pool architecture
- **Intelligent batching** with time-based and threshold-based triggers
- **Deduplication and change detection** to minimize redundant data
- **Compression support** (zlib, LZ4, JPEG passthrough)
- **Retry logic** with exponential backoff for Stags communication
- **Health monitoring** and statistics collection
- **Graceful shutdown** with proper resource cleanup
- **Docker support** for easy deployment

## Quick Start

### Prerequisites

- Go 1.21 or later
- Docker (optional)

### Build and Run

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
   ./relay
   ```

### Configuration

Configuration can be provided via:
- Environment variables (prefixed with `RELAY_`)
- Configuration file (`relay.yaml`)
- Command-line flags

#### Key Configuration Options

| Setting | Environment Variable | Default | Description |
|---------|---------------------|---------|-------------|
| Port | `RELAY_PORT` | `8080` | WebSocket listener port |
| Max Clients | `RELAY_MAX_CLIENTS` | `100` | Maximum concurrent connections |
| Worker Threads | `RELAY_WORKER_THREADS` | `4` | Processing worker count |
| Batch Interval | `RELAY_BATCH_INTERVAL_MS` | `100` | Batching interval in milliseconds |
| Snapshot Threshold | `RELAY_SNAPSHOT_THRESHOLD` | `0.1` | Change threshold (0.0-1.0) |
| Stags Endpoint | `RELAY_STAGS_ENDPOINT` | `http://localhost:8000/ingest` | Stags API endpoint |
| Log Level | `RELAY_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

### Docker Deployment

1. **Build Docker image:**
   ```bash
   docker build -t tabular-relay .
   ```

2. **Run container:**
   ```bash
   docker run -p 8080:8080 \
     -e RELAY_STAGS_ENDPOINT=https://your-stags.com/ingest \
     tabular-relay
   ```

## Testing

### Unit Tests
```bash
go test ./...
```

### Integration Test

The project includes a comprehensive test client that validates end-to-end functionality:

1. **Start the relay:**
   ```bash
   go run cmd/relay/main.go
   ```

2. **Run the test client:**
   ```bash
   # Using Go
   go run test_client.go
   
   # Or using the shell script
   ./run_test.sh
   ```

3. **Custom test parameters:**
   ```bash
   go run test_client.go \
     -relay-url ws://localhost:8080/ws/streamkit \
     -concurrent 5 \
     -interval 50ms \
     -verbose
   ```

The test client:
- Spins up a mock Stags server
- Sends 5 sample packets concurrently
- Validates HTTP 200 responses from Stags
- Reports pass/fail status with detailed metrics

### Test Data

Sample StreamKit packets are provided in `testdata/`:
- `sample_packet_1.json` - Mesh + Pose data
- `sample_packet_2.json` - Camera + Depth + Lighting data
- `sample_packet_3.json` - Multi-anchor mesh + Point cloud
- `sample_packet_4.json` - Pose with IMU data
- `sample_packet_5.json` - High-res camera + Depth

## StreamKit Protocol

The relay expects StreamKit packets in the following binary format:

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

Each stream contains:
```
[4 bytes] Metadata size (little-endian)
[N bytes] Metadata JSON
[M bytes] Compressed stream data
```

### Supported Stream Types

- **mesh**: 3D geometry with anchor transforms
- **camera**: RGB imagery with intrinsics
- **depth**: LiDAR depth maps with confidence
- **pose**: Device pose and IMU data
- **pointCloud**: 3D point clouds with colors
- **lighting**: Environmental lighting estimation

### Compression Support

- **none**: No compression
- **zlib**: Standard zlib compression
- **lz4**: High-speed LZ4 compression
- **jpeg**: Pass-through for camera streams

## API Endpoints

### WebSocket
- `GET /ws/streamkit` - StreamKit client connection

**Query Parameters:**
- `session_id`: Unique session identifier
- `device_id`: Device identifier

### HTTP
- `GET /health` - Health check and statistics

## Monitoring

The relay provides structured JSON logging with the following components:
- Connection events (connect, disconnect, errors)
- Packet processing metrics (parse time, transform time)
- Batch processing statistics (events per batch, success rate)
- Performance metrics (memory usage, processing latency)

Example log entry:
```json
{
  "timestamp": "2023-12-20T10:30:00Z",
  "level": "info",
  "component": "updater",
  "message": "Batch sent successfully",
  "batch_id": "batch_1703073000123",
  "event_count": 5,
  "duration_ms": 45.2
}
```

## Performance

Typical performance characteristics:
- **Throughput**: 1000+ packets/second per worker thread
- **Latency**: <10ms processing time per packet
- **Memory**: ~50MB baseline + ~1KB per active connection
- **CPU**: <20% on modern hardware under normal load

## Production Considerations

1. **Scaling**: Run multiple relay instances behind a load balancer
2. **Monitoring**: Integrate with Prometheus/Grafana for metrics
3. **Security**: Use TLS for WebSocket connections in production
4. **Backup**: Configure multiple Stags endpoints for redundancy
5. **Rate Limiting**: Implement client rate limiting if needed

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Check if relay is running on the correct port
   - Verify firewall settings

2. **Stags Communication Failures**
   - Verify Stags endpoint URL
   - Check network connectivity
   - Review Stags API logs

3. **High Memory Usage**
   - Reduce `RELAY_MAX_CLIENTS`
   - Decrease `RELAY_BATCH_INTERVAL_MS`
   - Check for connection leaks

4. **Processing Delays**
   - Increase `RELAY_WORKER_THREADS`
   - Reduce `RELAY_BATCH_INTERVAL_MS`
   - Monitor CPU usage

### Debug Mode

Enable debug logging for detailed troubleshooting:
```bash
export RELAY_LOG_LEVEL=debug
./relay
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

[Add your license information here]