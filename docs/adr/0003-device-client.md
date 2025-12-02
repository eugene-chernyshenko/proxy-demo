# ADR 0003: Device Client Architecture

## Status

Accepted

## Context

The proxy application supports an Outbound Pool system where devices can dynamically connect and be used for routing traffic. However, there was no client-side implementation for devices to connect to the proxy. Devices need to:

1. Register via HTTP API
2. Establish a reverse connection for tunneling SOCKS5 traffic
3. Send periodic heartbeats
4. Handle SOCKS5 requests from the proxy and proxy them to the internet

The implementation should follow the same modular architecture principles as the proxy server and reuse code where possible.

## Decision

We will implement a device client with the following architecture:

### Core Principles

1. **Modular Structure**: Separate `cmd/proxy/` and `cmd/device/` for multi-binary project
2. **Code Reuse**: Common SOCKS5 parser in `internal/protocol/socks5/` shared between server and client
3. **Encapsulation**: Client-side logic isolated in `internal/device/client/`, separate from server-side `internal/device/`
4. **Configuration**: JSON file + CLI flags (consistent with proxy)

### Architecture

```
cmd/
├── proxy/main.go          # Proxy server entry point
└── device/main.go         # Device client entry point

internal/
├── protocol/socks5/       # Shared SOCKS5 parser
│   ├── parser.go          # Address parsing
│   ├── request.go         # Request building
│   └── response.go       # Response building
└── device/
    ├── client/            # Client-side logic
    │   ├── client.go      # Main client orchestrator
    │   ├── registry.go    # HTTP API client
    │   └── reverse.go    # Reverse connection handler
    └── ...                # Server-side logic (existing)

config/
└── device_config.go       # Device configuration
```

### Components

1. **SOCKS5 Parser** (`internal/protocol/socks5/`):
   - `ParseAddress()` - Parse address from SOCKS5 request
   - `BuildRequest()` - Build SOCKS5 connection request
   - `BuildResponse()` - Build SOCKS5 response
   - Used by both `inbound/socks5.go` (server) and `internal/device/client/reverse.go` (client)

2. **Registry Client** (`internal/device/client/registry.go`):
   - `Register()` - Register device via HTTP API
   - `SendHeartbeat()` - Send heartbeat
   - `StartHeartbeat()` - Start periodic heartbeat

3. **Reverse Handler** (`internal/device/client/reverse.go`):
   - `Connect()` - Establish reverse connection
   - `HandleSOCKS5Requests()` - Process SOCKS5 requests through reverse connection
   - Uses shared SOCKS5 parser
   - Connects to target addresses via direct outbound
   - Forwards data between connections

4. **Device Client** (`internal/device/client/client.go`):
   - Orchestrates registry and reverse handler
   - Manages lifecycle (start/stop)
   - Handles reconnection logic

5. **Configuration** (`config/device_config.go`):
   - `DeviceConfig` struct
   - `LoadDeviceConfig()` - Load from JSON + CLI flags
   - Fields: ProxyHost, HTTPPort, ReversePort, DeviceID, Location, Tags, HeartbeatInterval

## Consequences

### Positive

- **Code Reuse**: SOCKS5 parsing logic shared between server and client
- **Clear Separation**: Client and server logic are clearly separated
- **Modularity**: Components are independent and testable
- **Consistency**: Configuration approach matches proxy server
- **Extensibility**: Easy to add new components or features

### Negative

- **Complexity**: Additional codebase to maintain
- **Reconnection Logic**: Device client needs to handle connection failures and reconnection

### Trade-offs

- **Shared Parser vs Duplication**: Chose shared parser to avoid code duplication and ensure consistency
- **Separate Package vs Same Package**: Chose `internal/device/client/` to clearly separate client-side logic from server-side logic in `internal/device/`

## Implementation Notes

- Device client automatically reconnects if reverse connection is lost
- Heartbeat runs in background goroutine
- SOCKS5 request handling runs in main goroutine after connection establishment
- Configuration can be provided via JSON file or CLI flags (CLI overrides file)

