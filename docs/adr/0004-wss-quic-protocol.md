# ADR 0004: WSS + QUIC Protocol for POP-Device Communication

## Status

Accepted

## Context

The previous protocol for POP-Device communication used HTTP API for registration/heartbeat and TCP reverse connections for data tunneling. This approach had several issues:

1. **NAT Traversal**: TCP reverse connections required devices to maintain persistent connections, which worked but was fragile
2. **Protocol Mixing**: Mixing plaintext HTTP with binary SOCKS5 on the same TCP connection caused buffering issues
3. **Synchronization**: Device client had to read SOCKS5 data immediately after sending token, leading to timing issues
4. **Scalability**: Single TCP connection per device limited concurrent requests

We needed a cleaner separation between control-plane and data-plane, with better NAT traversal support.

## Decision

We will replace the HTTP + TCP reverse connection protocol with:

1. **Control-plane**: WebSocket Secure (WSS) on port 443/TLS

   - Device registration
   - Commands from POP to Device (open_tcp/udp, close)
   - Heartbeats and load reports
   - Protocol Buffers for message serialization

2. **Data-plane**: QUIC on port 443/UDP
   - One QUIC connection per device
   - Each `conn_id` maps to a separate QUIC stream
   - TCP traffic through streams
   - UDP traffic through datagrams (future)
   - IP-migration support (architecture ready, implementation deferred)

## Architecture

### Control-Plane (WSS)

**POP Side:**

- `internal/device/wss/server.go` - WSS server
- `internal/device/wss/handler.go` - Message handling (register, heartbeat, load reports, command responses)
- `internal/device/wss/commands.go` - Command sending to devices

**Device Side:**

- `internal/device/client/wss/client.go` - WSS client
- `internal/device/client/wss/handler.go` - Command handling (open_tcp/udp, close)

**Protocol:**

- Messages encoded as Protocol Buffers
- Format: `[4-byte length (big-endian)][protobuf message]`
- Binary WebSocket messages

### Data-Plane (QUIC)

**POP Side:**

- `internal/device/quic/server.go` - QUIC server
- Accepts QUIC connections from devices
- Maps `conn_id` to QUIC streams
- Used by `outbound/quic.go` for proxying

**Device Side:**

- `internal/device/client/quic/client.go` - QUIC client
- `internal/device/client/quic/stream_handler.go` - Stream handling and TCP proxying

**Protocol:**

- Device establishes QUIC connection after WSS registration
- POP sends `OpenTCP` command via WSS with `conn_id` and `target_address`
- Device opens QUIC stream and proxies TCP traffic
- Each `conn_id` = one QUIC stream

### Device Structure Updates

**Removed:**

- `ReverseConn net.Conn`
- `Token []byte` (no longer needed)

**Added:**

- `WSSConn *websocket.Conn` - WSS control connection
- `QUICConn *quic.Conn` - QUIC data connection
- `Streams map[string]*quic.Stream` - Active streams (conn_id → stream)

### Configuration Updates

**OutboundPoolConfig:**

- Removed: `HTTPPort`, `ReversePort`
- Added: `WSSPort` (default: 443), `QUICPort` (default: 443), `TLS *TLSConfig`

**DeviceConfig:**

- Removed: `HTTPPort`, `ReversePort`
- Added: `WSSPort` (default: 443), `QUICPort` (default: 443), `TLSSkipVerify bool`

## Consequences

### Positive

- **Better NAT Traversal**: QUIC works better through NAT than TCP reverse connections
- **Clean Separation**: Control-plane (WSS) and data-plane (QUIC) are clearly separated
- **Concurrent Requests**: Multiple QUIC streams allow concurrent requests per device
- **Protocol Buffers**: Type-safe, efficient message serialization
- **Future-Proof**: Architecture supports IP-migration and UDP datagrams

### Negative

- **Complexity**: More complex than HTTP + TCP
- **Dependencies**: Requires `nhooyr.io/websocket` and `quic-go`
- **TLS**: QUIC requires TLS (though can be disabled for testing)

### Trade-offs

- **WSS vs HTTP**: Chose WSS for bidirectional communication and better integration with WebSocket infrastructure
- **QUIC vs TCP**: Chose QUIC for better NAT traversal, multiplexing, and future IP-migration support
- **Protocol Buffers vs JSON**: Chose Protocol Buffers for efficiency and type safety, though adds complexity

## Implementation Notes

- WSS server runs on port 443 (or configured port) with optional TLS
- QUIC server runs on port 443 (or configured port) with optional TLS
- Device connects to WSS first, then QUIC after registration
- Commands sent via WSS, data flows through QUIC streams
- Device automatically reconnects if WSS or QUIC connection is lost
- Heartbeat runs in background goroutine via WSS

## Future Enhancements

- IP-migration support for QUIC (architecture ready)
- UDP proxying through QUIC datagrams
- Load balancing based on device metrics
- Connection pooling and stream reuse
