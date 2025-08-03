# Logging Field Standards

This document defines the standardized field names and data types for consistent logging across all components in the StreamWeaver application.

## Overview

The StreamWeaver application uses structured logging with standardized field names to ensure consistency across all services. This approach enables better log analysis, filtering, and debugging capabilities.

This document serves as the technical specification for field standards. For a quick reference, see the [Field Naming Reference](../FIELD_NAMING_REFERENCE.md). For detailed usage guidelines, see the [Field Naming Guide](field_naming_guide.md).

## Field Naming Convention

All field names must use `snake_case` for consistency and readability.

## Standardized Fields

### Request/Response Fields
- `correlation_id` (string): Unique identifier for request tracing
- `method` (string): HTTP method (GET, POST, etc.)
- `url` (string): Full URL for HTTP operations
- `path` (string): URL path component
- `status_code` (int): HTTP status code
- `duration` (time.Duration): Operation duration
- `client_ip` (string): Client IP address
- `user_agent` (string): Client user agent string

### Component/Service Fields
- `component` (string): Component/service that generated the log
- `operation` (string): Specific operation being performed
- `error` (string): Error message when operation fails
- `error_type` (string): Classification of error type

### Stream-Related Fields
- `stream_id` (string): Identifier for stream-related operations
- `channel_name` (string): IPTV channel name
- `playlist_url` (string): M3U playlist URL
- `segment_url` (string): Media segment URL
- `buffer_status` (string): Status of stream buffering
- `client_count` (int): Number of connected clients to a stream
- `stream_type` (string): Type of stream (m3u8, media, etc.)

### Load Balancer Fields
- `load_balancer_result` (string): Result from load balancing decisions
- `target_url` (string): Selected target URL from load balancer
- `attempt_count` (int): Number of attempts made
- `failover_reason` (string): Reason for failover

### Performance Fields
- `latency` (time.Duration): Network latency
- `bytes_transferred` (int64): Number of bytes transferred
- `response_size` (int64): Size of response in bytes
- `request_size` (int64): Size of request in bytes

### System Fields
- `memory_usage` (int64): Memory usage in bytes
- `cpu_usage` (float64): CPU usage percentage
- `goroutine_count` (int): Number of active goroutines

## Field Usage Guidelines

1. **Consistency**: Always use the exact field name as defined above
2. **Data Types**: Maintain consistent data types for each field across all components
3. **Null Values**: Use appropriate zero values for optional fields
4. **Nesting**: Avoid deeply nested field structures; prefer flat field names
5. **Units**: Include units in field names when ambiguous (e.g., `duration_ms` vs `duration`)
## Migration Guidelines

When updating existing log statements:

1. Replace non-standard field names with standardized equivalents
2. Ensure data types match the standard
3. Update any related documentation or comments
4. Test that log parsing and analysis tools still work correctly

### Common Field Migrations

| Current Field | Standard Field | Notes |
|---------------|----------------|-------|
| `url` | `url` | ✅ Already correct |
| `target_url` | `target_url` | ✅ Already correct for load balancer context |
| `actual_url` | `url` | Use standard `url` field |
| `elapsed` | `duration` | Use standard `duration` field |
| `attempt` | `attempt_count` | Be more explicit |
| `client_number` | `client_count` | Use count semantics |
| `remaining_clients` | `client_count` | Use standard count field |

## Examples

### Before (Non-standard)
```go
logger.InfoEvent().
    Str("component", "LoadBalancerInstance").
    Str("url", targetURL).
    Int("attempt", attemptNum).
    Msg("Trying URL")
```

### After (Standardized)
```go
logger.InfoEvent().
    Str("component", "LoadBalancerInstance").
    Str("target_url", targetURL).
    Int("attempt_count", attemptNum).
    Str("operation", "url_validation").
    Msg("Trying URL")
```

### Stream Context Example

### Before
```go
logger.InfoEvent().
    Str("component", "StreamHandler").
    Str("actual_url", actualURL).
    Int("remaining_clients", clientCount).
    Msg("Client disconnected")
```

### After (Standardized)
```go
logger.InfoEvent().
    Str("component", "StreamHandler").
    Str("url", actualURL).
    Int("client_count", clientCount).
    Str("operation", "client_disconnect").
    Msg("Client disconnected")
```

## Implementation Priority

1. **High Priority**: Fix inconsistent field names that appear frequently
   - Standardize `attempt` → `attempt_count`
   - Standardize `elapsed` → `duration`
   - Add missing `operation` fields

2. **Medium Priority**: Add missing standardized fields
   - Add `error_type` where appropriate
   - Add `operation` context to existing logs

3. **Low Priority**: Enhance with additional context
   - Add performance metrics where relevant
   - Add system metrics for debugging