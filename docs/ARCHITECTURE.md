# Technical Architecture

## System Design

### Overview
The Walrus-Rclone MVP demonstrates a clean integration between local file systems and Walrus decentralized storage through a command-line interface. The architecture prioritizes simplicity, reliability, and extensibility.

### Component Architecture

```
┌──────────────────┐
│   CLI Interface  │
│  (walrus-cli)    │
└────────┬─────────┘
         │
    ┌────▼─────┐
    │  Backend │
    │   Layer  │
    └────┬─────┘
         │
    ┌────▼────────┐
    │ Walrus HTTP │
    │   Client    │
    └────┬────────┘
         │
    ┌────▼──────────┐
    │ Walrus Network│
    │  (Testnet)    │
    └───────────────┘
```

## Core Components

### 1. Walrus HTTP Client (`backend/client.go`)
**Purpose**: Handles all communication with Walrus storage network

**Key Functions**:
- `StoreBlob()`: Uploads data to Walrus publisher
- `RetrieveBlob()`: Downloads data from Walrus aggregator
- `GetBlobStatus()`: Checks blob availability
- `EstimateStorageCost()`: Calculates storage costs

**Design Decisions**:
- Uses standard HTTP client with 30s timeout
- Implements retry logic at application level
- Returns structured responses with error handling

### 2. Configuration Management (`backend/config.go`)
**Purpose**: Manages application settings and user preferences

**Features**:
- YAML-based configuration
- Default values for testnet
- Multiple config file locations
- Environment variable support (future)

**Configuration Structure**:
```yaml
walrus:
  aggregator_url: "..."
  publisher_url: "..."
  epochs: 5
  wallet:
    private_key: ""
```

### 3. File System Interface (`backend/simple.go`)
**Purpose**: Provides abstraction layer for file operations

**Key Components**:
- `SimpleFs`: Main filesystem struct
- `SimpleFileIndex`: Maps filenames to blob IDs
- Thread-safe operations with RWMutex

**Index Management**:
- JSON file stored at `~/.walrus-simple-index.json`
- Tracks: blob ID, size, modification time, expiry epoch
- Atomic updates with file locking

### 4. CLI Application (`cmd/walrus-cli/main.go`)
**Purpose**: User interface for all operations

**Commands**:
- `init`: Initialize configuration
- `upload`: Store files in Walrus
- `download`: Retrieve files from Walrus
- `list`: Show indexed files
- `cost`: Estimate storage costs

**Features**:
- Progress indicators
- Dry-run mode
- Flexible output paths
- Human-readable formatting

## Data Flow

### Upload Process
```
1. Read file from disk
2. Calculate storage cost
3. Display cost estimate
4. Upload to Walrus publisher
   - Publisher encodes data
   - Distributes to storage nodes
   - Returns blob ID
5. Update local index
6. Save index to disk
```

### Download Process
```
1. Look up blob ID in index
2. Request from Walrus aggregator
   - Aggregator fetches slivers
   - Reconstructs original data
3. Write data to disk
4. Verify integrity
```

## Storage Model

### Blob Management
- Content-addressed storage using blob IDs
- Immutable once stored
- Configurable expiry (epochs)
- No directory structure (flat namespace)

### Index Design
```json
{
  "files": {
    "document.pdf": {
      "blob_id": "x7K9mP3nQ5...",
      "size": 2457600,
      "mod_time": "2024-01-15T10:30:00Z",
      "expiry_epoch": 125
    }
  }
}
```

## Network Communication

### HTTP Endpoints Used
- **Publisher**: `PUT /v1/blobs?epochs=N`
- **Aggregator**: `GET /v1/blobs/{blobId}`
- **Status**: `HEAD /v1/blobs/{blobId}`

### Error Handling
- HTTP status code validation
- Timeout management (30 seconds)
- Retry logic (3 attempts)
- Detailed error messages

## Security Considerations

### Current Implementation
- HTTPS for all communications
- Local index file permissions (0644)
- No sensitive data in logs
- Optional wallet key configuration

### Future Enhancements
- Seal encryption integration
- Access control policies
- Key server communication
- Secure key management

## Performance Characteristics

### Limitations (MVP)
- Single-threaded uploads
- 100MB file size limit
- No chunking for large files
- Sequential operations

### Optimizations
- Efficient memory usage (streaming)
- Minimal disk I/O
- Cached configuration
- Quick index lookups

## Extensibility

### Plugin Points
- Custom storage backends
- Alternative index formats
- Encryption providers
- Progress callbacks

### Future Integration
- Rclone native backend
- Airbyte connector
- Seal encryption
- Parallel transfers

## Testing Strategy

### Unit Tests
- Client functions
- Cost calculations
- Index operations
- Configuration parsing

### Integration Tests
- End-to-end upload/download
- Network failure scenarios
- Index consistency
- Cost estimation accuracy

### Performance Tests
- Various file sizes
- Network latency simulation
- Concurrent operations
- Memory profiling

## Deployment

### Requirements
- Go 1.21+
- Internet connection
- 10MB disk space
- Linux/macOS/Windows

### Installation
```bash
go build -o walrus-cli cmd/walrus-cli/main.go
./walrus-cli init
```

### Configuration
- Default testnet endpoints
- Optional wallet configuration
- Custom epoch settings
- Index location override

## Monitoring

### Metrics
- Upload/download success rates
- Transfer speeds
- Storage costs
- Error frequencies

### Logging
- Structured log output
- Debug mode available
- Error tracking
- Performance timing

## Conclusion

This architecture provides a solid foundation for Walrus integration while maintaining simplicity and extensibility. The modular design allows for easy enhancement and integration with other tools while demonstrating the core capabilities of decentralized storage.