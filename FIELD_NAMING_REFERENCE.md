# Field Naming Reference for StreamWeaver Logging

This document provides a quick reference for standardized field names used in logging across the StreamWeaver application.

## Quick Reference

### Request/Response Fields
| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `correlation_id` | string | Unique identifier for request tracing | `"req-12345-abcde"` |
| `method` | string | HTTP method | `"GET"`, `"POST"` |
| `url` | string | Full URL for HTTP operations | `"http://example.com/api/streams"` |
| `path` | string | URL path component | `"/api/streams"` |
| `status_code` | int | HTTP status code | `200`, `404`, `500` |
| `duration` | time.Duration | Operation duration | `time.Millisecond * 150` |
| `client_ip` | string | Client IP address | `"192.168.1.100"` |
| `user_agent` | string | Client user agent string | `"VLC/3.0.16"` |

### Component/Service Fields
| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `component` | string | Component/service that generated the log | `"StreamHandler"`, `"LoadBalancer"` |
| `operation` | string | Specific operation being performed | `"client_connect"`, `"url_validation"` |
| `error` | string | Error message when operation fails | `"connection timeout"` |
| `error_type` | string | Classification of error type | `"network_error"`, `"validation_error"` |

### Stream-Related Fields
| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `stream_id` | string | Identifier for stream-related operations | `"channel-123"` |
| `channel_name` | string | IPTV channel name | `"HBO"`, `"CNN"` |
| `playlist_url` | string | M3U playlist URL | `"http://server.com/playlist.m3u8"` |
| `segment_url` | string | Media segment URL | `"http://server.com/segment001.ts"` |
| `buffer_status` | string | Status of stream buffering | `"buffering"`, `"ready"`, `"empty"` |
| `client_count` | int | Number of connected clients to a stream | `5`, `12` |
| `stream_type` | string | Type of stream | `"m3u8"`, `"media"`, `"live"` |

### Load Balancer Fields
| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `load_balancer_result` | string | Result from load balancing decisions | `"success"`, `"all_failed"` |
| `target_url` | string | Selected target URL from load balancer | `"http://server1.com/stream"` |
| `attempt_count` | int | Number of attempts made | `1`, `3`, `5` |
| `failover_reason` | string | Reason for failover | `"timeout"`, `"404_error"` |

### Performance Fields
| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `latency` | time.Duration | Network latency | `time.Millisecond * 50` |
| `bytes_transferred` | int64 | Number of bytes transferred | `1048576` |
| `response_size` | int64 | Size of response in bytes | `2048` |
| `request_size` | int64 | Size of request in bytes | `512` |

### System Fields
| Field Name | Type | Description | Example |
|------------|------|-------------|---------|
| `memory_usage` | int64 | Memory usage in bytes | `67108864` |
| `cpu_usage` | float64 | CPU usage percentage | `75.5` |
| `goroutine_count` | int | Number of active goroutines | `25` |

## Using Standardized Fields

### With StandardFields Helper (Recommended)
```go
import "m3u-stream-merger/logger"

// Create standardized fields
fields := logger.NewStandardFields().
    WithComponent("StreamHandler").
    WithOperation("client_connect").
    WithStreamID("channel-123").
    WithClientCount(5).
    WithDuration(time.Millisecond * 200)

// Use with logger
requestLogger := logger.Default.WithStandardFields(fields)
requestLogger.InfoEvent().Msg("Client connected successfully")

// Or apply to an event directly
logger.Default.InfoEvent().
    ApplyToEvent(fields.ApplyToEvent).
    Msg("Client connected successfully")
```

### With Direct Event Fields
```go
logger.Default.InfoEvent().
    Str("component", "StreamHandler").
    Str("operation", "client_connect").
    Str("stream_id", "channel-123").
    Int("client_count", 5).
    Dur("duration", time.Millisecond * 200).
    Msg("Client connected successfully")
```

## Common Patterns by Component

### Load Balancer Logs
```go
// URL validation attempt
fields := logger.NewStandardFields().
    WithComponent("LoadBalancerInstance").
    WithOperation("url_validation").
    WithTargetURL(url).
    WithAttemptCount(attemptNum).
    WithStreamID(streamID)

logger.InfoEvent().ApplyFields(fields).Msg("Trying URL")
```

### Stream Handler Logs
```go
// Client connection
fields := logger.NewStandardFields().
    WithComponent("StreamHandler").
    WithOperation("client_connect").
    WithStreamID(streamID).
    WithClientIP(clientIP).
    WithClientCount(newCount)

logger.InfoEvent().ApplyFields(fields).Msg("Client connected")
```

### HTTP Handler Logs
```go
// Request processing
fields := logger.NewStandardFields().
    WithComponent("M3UHTTPHandler").
    WithOperation("process_request").
    WithCorrelationID(correlationID).
    WithMethod(r.Method).
    WithURL(r.URL.String()).
    WithClientIP(r.RemoteAddr)

logger.InfoEvent().ApplyFields(fields).Msg("Processing request")
```

## Migration Examples

### Before (Non-standard)
```go
logger.InfoEvent().
    Str("component", "LoadBalancer").
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

## Best Practices

1. **Always use standardized field names** - This ensures consistency across the application
2. **Include component and operation fields** - This helps with filtering and debugging
3. **Use appropriate data types** - Follow the type specifications in the reference
4. **Add correlation IDs for requests** - This enables request tracing across components
5. **Include performance metrics** - Add duration, counts, and sizes where relevant
6. **Use descriptive operation names** - Operations should clearly indicate what's happening

## Validation

To ensure your logging follows the standards:

1. **Run tests**: The field standardization tests will catch inconsistencies
2. **Code review**: Check that new logging code uses standardized fields
3. **Log analysis**: Use log aggregation tools to verify consistent field usage

## Adding New Fields

When adding new standardized fields:

1. Update [`logger/fields.go`](logger/fields.go) with new methods
2. Update [`logger/field_standards.md`](logger/field_standards.md) with the new field specification
3. Update this reference document
4. Add tests in [`logger/fields_test.go`](logger/fields_test.go)
5. Update existing code to use the new fields where appropriate

## Related Documentation

- [Field Standards Specification](logger/field_standards.md) - Detailed technical specification
- [Logger Enhancement Plan](logger_enhancement_plan_enhanced.md) - Overall logging strategy
- [Logging Enhancement Summary](logging_enhancement_summary_enhanced.md) - Implementation overview