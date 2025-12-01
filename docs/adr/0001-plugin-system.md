# ADR 0001: Plugin System Architecture

## Status

Accepted

## Context

The proxy application needs to support different deployment scenarios:

- **Edge POP**: Entry point for clients, needs to count traffic per client
- **Supply POP**: Connection point for supply devices, needs to count traffic per supply device
- **Client**: Simple proxy usage

The application should be modular and extensible without modifying core code. Business concepts like "edge", "supply", and "POP" should not be part of the core application - only abstractions of inbound/outbound with identifiers.

## Decision

We will implement a plugin system with the following architecture:

### Core Principles

1. **Abstraction**: The core application only knows about:
   - Inbound with identifier (InboundID)
   - Outbound with identifier (OutboundID) and type (direct/socks5)
2. **Business concepts** (edge/supply/POP) exist only:

   - In plugin configuration
   - In plugin interpretation of identifiers
   - Not in core application code

3. **Router as internal component**: Router is an internal, always-enabled component (not a plugin) that can select between current outbound or another outbound.

### Architecture Components

#### 1. Plugin Infrastructure (`internal/plugin/`)

- **ConnectionContext**: Carries connection metadata (InboundID, OutboundID, traffic stats, etc.)
- **Plugin Interfaces**: Base interfaces for different plugin types
  - `InboundPlugin`: Handles inbound events
  - `OutboundPlugin`: Handles outbound events
  - `TrafficPlugin`: Counts traffic
- **PluginManager**: Manages plugin registration and hook invocation

#### 2. Router (`internal/router/`)

- **Router Interface**: Selects outbound for target address
  - Returns `outboundID` for existing outbound from pool
  - Returns `outboundConfig` for new outbound
  - Returns `nil, nil` for current outbound
- **StaticRouter**: Always returns `nil, nil` (use current outbound)
- Router is always enabled, used directly in `proxy/core.go`

#### 3. Traffic Counter Plugins (`internal/plugins/traffic/`)

- **InboundCounter**: Counts traffic by InboundID
- **OutboundCounter**: Counts traffic by OutboundID
- Both use in-memory storage (default)

### Integration Points

1. **proxy/core.go**:

   - Creates ConnectionContext
   - Calls Router.SelectOutbound before establishing outbound connection
   - Calls PluginManager hooks at appropriate points
   - Uses CopyDataWithCounting for traffic measurement

2. **inbound/socks5.go**:

   - Creates ConnectionContext
   - Passes context to handler

3. **main.go**:
   - Initializes Router (Static)
   - Initializes PluginManager
   - Loads plugins from configuration
   - Passes Router and PluginManager to handler

### Configuration

```json
{
  "inbound": {
    "type": "socks5",
    "port": 1080,
    "id": "inbound-1"
  },
  "outbound": {
    "type": "socks5",
    "proxy_address": "127.0.0.1:1081",
    "id": "outbound-1"
  },
  "plugins": {
    "traffic_inbound": {
      "enabled": true
    },
    "traffic_outbound": {
      "enabled": true
    }
  }
}
```

## Consequences

### Positive

- **Modularity**: Plugins can be added/removed without core changes
- **Extensibility**: Easy to add new plugins (auth, rate limiting, etc.)
- **Abstraction**: Core doesn't know about business concepts
- **Testability**: Each component can be tested independently
- **Backward compatibility**: Works without plugins

### Negative

- **Complexity**: Additional abstraction layer
- **Performance**: Hook calls add overhead (minimal, async where possible)
- **Configuration**: More complex configuration structure

### Future Extensions

- **Outbound pool with heartbeats**: Supply devices connect, send heartbeats, Router selects them
- **Dynamic plugin loading**: Load plugins without restart
- **External Router**: Support for external router service
- **Authentication plugins**: Extract InboundID from tokens
- **Rate limiting plugins**: Traffic shaping per InboundID/OutboundID
- **Metrics export**: Export statistics to monitoring systems

## Alternatives Considered

1. **Router as Plugin**: Rejected - Router is core functionality, should be internal
2. **Business concepts in core**: Rejected - Violates abstraction principle
3. **Separate services**: Rejected - Too complex for current needs
4. **Configuration-based routing**: Accepted - Simple, extensible

## Notes

- Router is always enabled (no `enabled` flag)
- Storage defaults to memory (no `storage` config needed)
- Identifiers (inbound_id, outbound_id) are optional but recommended for plugins
