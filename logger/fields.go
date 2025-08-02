package logger

import (
	"fmt"
	"time"
)

// StandardFields provides type-safe methods for adding standardized fields to log entries
type StandardFields struct {
	fields map[string]interface{}
}

// NewStandardFields creates a new StandardFields instance
func NewStandardFields() *StandardFields {
	return &StandardFields{
		fields: make(map[string]interface{}, 16), // Pre-allocate for typical field count
	}
}

// Fields returns the underlying fields map
func (sf *StandardFields) Fields() map[string]interface{} {
	return sf.fields
}

// Request/Response Fields

// WithCorrelationID adds a correlation_id field
func (sf *StandardFields) WithCorrelationID(id string) *StandardFields {
	sf.fields["correlation_id"] = id
	return sf
}

// WithMethod adds a method field (HTTP method)
func (sf *StandardFields) WithMethod(method string) *StandardFields {
	sf.fields["method"] = method
	return sf
}

// WithURL adds a url field
func (sf *StandardFields) WithURL(url string) *StandardFields {
	sf.fields["url"] = url
	return sf
}

// WithPath adds a path field
func (sf *StandardFields) WithPath(path string) *StandardFields {
	sf.fields["path"] = path
	return sf
}

// WithStatusCode adds a status_code field
func (sf *StandardFields) WithStatusCode(code int) *StandardFields {
	sf.fields["status_code"] = code
	return sf
}

// WithDuration adds a duration field
func (sf *StandardFields) WithDuration(duration time.Duration) *StandardFields {
	sf.fields["duration"] = duration
	return sf
}

// WithClientIP adds a client_ip field
func (sf *StandardFields) WithClientIP(ip string) *StandardFields {
	sf.fields["client_ip"] = ip
	return sf
}

// WithUserAgent adds a user_agent field
func (sf *StandardFields) WithUserAgent(userAgent string) *StandardFields {
	sf.fields["user_agent"] = userAgent
	return sf
}

// Component/Service Fields

// WithComponent adds a component field
func (sf *StandardFields) WithComponent(component string) *StandardFields {
	sf.fields["component"] = component
	return sf
}

// WithOperation adds an operation field
func (sf *StandardFields) WithOperation(operation string) *StandardFields {
	sf.fields["operation"] = operation
	return sf
}

// WithError adds an error field
func (sf *StandardFields) WithError(err string) *StandardFields {
	sf.fields["error"] = err
	return sf
}

// WithErrorType adds an error_type field
func (sf *StandardFields) WithErrorType(errorType string) *StandardFields {
	sf.fields["error_type"] = errorType
	return sf
}

// Stream-Related Fields

// WithStreamID adds a stream_id field
func (sf *StandardFields) WithStreamID(id string) *StandardFields {
	sf.fields["stream_id"] = id
	return sf
}

// WithChannelName adds a channel_name field
func (sf *StandardFields) WithChannelName(name string) *StandardFields {
	sf.fields["channel_name"] = name
	return sf
}

// WithPlaylistURL adds a playlist_url field
func (sf *StandardFields) WithPlaylistURL(url string) *StandardFields {
	sf.fields["playlist_url"] = url
	return sf
}

// WithSegmentURL adds a segment_url field
func (sf *StandardFields) WithSegmentURL(url string) *StandardFields {
	sf.fields["segment_url"] = url
	return sf
}

// WithBufferStatus adds a buffer_status field
func (sf *StandardFields) WithBufferStatus(status string) *StandardFields {
	sf.fields["buffer_status"] = status
	return sf
}

// WithClientCount adds a client_count field
func (sf *StandardFields) WithClientCount(count int) *StandardFields {
	sf.fields["client_count"] = count
	return sf
}

// WithStreamType adds a stream_type field
func (sf *StandardFields) WithStreamType(streamType string) *StandardFields {
	sf.fields["stream_type"] = streamType
	return sf
}

// Load Balancer Fields

// WithLoadBalancerResult adds a load_balancer_result field
func (sf *StandardFields) WithLoadBalancerResult(result string) *StandardFields {
	sf.fields["load_balancer_result"] = result
	return sf
}

// WithTargetURL adds a target_url field
func (sf *StandardFields) WithTargetURL(url string) *StandardFields {
	sf.fields["target_url"] = url
	return sf
}

// WithAttemptCount adds an attempt_count field
func (sf *StandardFields) WithAttemptCount(count int) *StandardFields {
	sf.fields["attempt_count"] = count
	return sf
}

// WithFailoverReason adds a failover_reason field
func (sf *StandardFields) WithFailoverReason(reason string) *StandardFields {
	sf.fields["failover_reason"] = reason
	return sf
}

// Performance Fields

// WithLatency adds a latency field
func (sf *StandardFields) WithLatency(latency time.Duration) *StandardFields {
	sf.fields["latency"] = latency
	return sf
}

// WithBytesTransferred adds a bytes_transferred field
func (sf *StandardFields) WithBytesTransferred(bytes int64) *StandardFields {
	sf.fields["bytes_transferred"] = bytes
	return sf
}

// WithResponseSize adds a response_size field
func (sf *StandardFields) WithResponseSize(size int64) *StandardFields {
	sf.fields["response_size"] = size
	return sf
}

// WithRequestSize adds a request_size field
func (sf *StandardFields) WithRequestSize(size int64) *StandardFields {
	sf.fields["request_size"] = size
	return sf
}

// System Fields

// WithMemoryUsage adds a memory_usage field
func (sf *StandardFields) WithMemoryUsage(usage int64) *StandardFields {
	sf.fields["memory_usage"] = usage
	return sf
}

// WithCPUUsage adds a cpu_usage field
func (sf *StandardFields) WithCPUUsage(usage float64) *StandardFields {
	sf.fields["cpu_usage"] = usage
	return sf
}

// WithGoroutineCount adds a goroutine_count field
func (sf *StandardFields) WithGoroutineCount(count int) *StandardFields {
	sf.fields["goroutine_count"] = count
	return sf
}

// WithTime adds a time field
func (sf *StandardFields) WithTime(key string, t time.Time) *StandardFields {
	sf.fields[key] = t
	return sf
}

// Generic field method for custom fields that follow the standard
func (sf *StandardFields) WithField(key string, value interface{}) *StandardFields {
	sf.fields[key] = value
	return sf
}

// Helper methods to apply standard fields to Event interface

// ApplyToEvent applies all standardized fields to an Event
func (sf *StandardFields) ApplyToEvent(event Event) Event {
	for key, value := range sf.fields {
		switch v := value.(type) {
		case string:
			event = event.Str(key, v)
		case int:
			event = event.Int(key, v)
		case int64:
			event = event.Int64(key, v)
		case float64:
			event = event.Float64(key, v)
		case bool:
			event = event.Bool(key, v)
		case time.Duration:
			event = event.Dur(key, v)
		case time.Time:
			event = event.Time(key, v)
		default:
			// For other types, convert to string
			event = event.Str(key, fmt.Sprintf("%v", v))
		}
	}
	return event
}