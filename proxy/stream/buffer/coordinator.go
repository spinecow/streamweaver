package buffer

import (
	"container/ring"
	"context"
	"errors"
	"fmt"
	"io"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy"
	"m3u-stream-merger/proxy/loadbalancer"
	"m3u-stream-merger/proxy/stream/config"
	"m3u-stream-merger/store"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/bytebufferpool"
)

var (
	ErrStreamClosed   = errors.New("stream is closed")
	ErrStreamDraining = errors.New("stream is draining")
)

// ChunkData holds a chunk of streamed data along with metadata.
type ChunkData struct {
	Buffer    *bytebufferpool.ByteBuffer
	Error     error
	Status    int
	Timestamp time.Time

	seq int64 // unexported sequence number for internal tracking.
}

// newChunkData creates a new chunk with a fresh ByteBuffer.
func newChunkData() *ChunkData {
	return &ChunkData{
		Buffer: bytebufferpool.Get(),
		seq:    0,
	}
}

// Reset resets the chunk. It returns the underlying buffer
// to the pool, obtains a new one, and clears all metadata.
// Once Reset is called the caller Must not use the old buffer.
func (c *ChunkData) Reset() {
	if c.Buffer != nil {
		c.Buffer.Reset()
		bytebufferpool.Put(c.Buffer)
	}
	c.Buffer = bytebufferpool.Get()
	c.Error = nil
	c.Status = 0
	c.Timestamp = time.Time{}
	c.seq = 0
}

// Internal state constants.
const (
	stateActive int32 = iota
	stateDraining
	stateClosed
)

type StreamCoordinator struct {
	Buffer       *ring.Ring
	Mu           sync.RWMutex
	broadcast    chan struct{}
	ClientCount  int32
	WriterCtx    context.Context
	WriterCancel context.CancelFunc
	WriterChan   chan struct{}
	WriterCtxMu  sync.Mutex
	WriterActive atomic.Bool

	WriterRespHeader atomic.Pointer[http.Header]
	respHeaderSet    chan struct{}
	m3uHeaderSet     atomic.Bool

	LastError atomic.Value
	logger    logger.Logger
	config    *config.StreamConfig
	cm        *store.ConcurrencyManager
	streamID  string
	actualURL string

	InitializationMu sync.Mutex

	// state represents active, draining, or closed.
	state int32

	LBResultOnWrite atomic.Pointer[loadbalancer.LoadBalancerResult]

	// writeSeq is an atomic counter to track the order of chunks.
	writeSeq int64
}

// SetActualURL sets the actual stream URL for this coordinator
func (c *StreamCoordinator) SetActualURL(url string) {
	c.Mu.Lock()
	defer c.Mu.Unlock()
	if c.actualURL == "" {
		c.actualURL = url
	}
}

// GetActualURL returns the actual stream URL for this coordinator
func (c *StreamCoordinator) GetActualURL() string {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	return c.actualURL
}

// subscribe returns the current broadcast channel.
func (c *StreamCoordinator) subscribe() <-chan struct{} {
	c.Mu.RLock()
	ch := c.broadcast
	c.Mu.RUnlock()
	return ch
}

// notifySubscribers closes the current broadcast channel and
// creates a new one so waiting clients can be notified.
func (c *StreamCoordinator) notifySubscribers() {
	c.Mu.Lock()
	close(c.broadcast)
	c.broadcast = make(chan struct{})
	c.Mu.Unlock()
}

func NewStreamCoordinator(streamID string, config *config.StreamConfig, cm *store.ConcurrencyManager, logger logger.Logger) *StreamCoordinator {
	logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("stream_id", streamID).
		Int("buffer_size", config.SharedBufferSize).
		Int("chunk_size", config.ChunkSize).
		Msg("Initializing new StreamCoordinator")

	r := ring.New(config.SharedBufferSize)
	for i := 0; i < config.SharedBufferSize; i++ {
		r.Value = newChunkData()
		r = r.Next()
	}

	coord := &StreamCoordinator{
		Buffer:        r,
		WriterChan:    make(chan struct{}, 1),
		logger:        logger,
		config:        config,
		cm:            cm,
		streamID:      streamID,
		actualURL:     "", // Will be set when the first client connects
		broadcast:     make(chan struct{}),
		respHeaderSet: make(chan struct{}),
	}
	atomic.StoreInt32(&coord.state, stateActive)
	coord.LastError.Store((*ChunkData)(nil))

	logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("stream_id", streamID).
		Int("buffer_size", config.SharedBufferSize).
		Int("chunk_size", config.ChunkSize).
		Msg("StreamCoordinator initialized successfully")
	return coord
}

func (c *StreamCoordinator) WaitHeaders(ctx context.Context) {
	select {
	case <-c.respHeaderSet:
	case <-ctx.Done():
	}
}

// GetWriterLBResult returns the load balancer result for the current writer call.
func (c *StreamCoordinator) GetWriterLBResult() *loadbalancer.LoadBalancerResult {
	return c.LBResultOnWrite.Load()
}

// RegisterClient registers a new client and returns an error if the stream
// is no longer active.
func (c *StreamCoordinator) RegisterClient() error {
	registrationStartTime := time.Now()
	state := atomic.LoadInt32(&c.state)

	// If stream is closed but there are no clients, allow reset
	if state != stateActive && atomic.LoadInt32(&c.ClientCount) == 0 {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "reset_stream_state").
			Str("stream_id", c.streamID).
			Int("previous_state", int(state)).
			Msg("Resetting closed stream to active state")
		atomic.StoreInt32(&c.state, stateActive)
	}

	count := atomic.AddInt32(&c.ClientCount, 1)
	registrationDuration := time.Since(registrationStartTime)

	c.logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("operation", "register_client").
		Str("stream_id", c.streamID).
		Int("client_count", int(count)).
		Dur("registration_duration", registrationDuration).
		Msg("Client registered")
	return nil
}

// UnregisterClient unregisters a client and cleans up resources if it was the last.
func (c *StreamCoordinator) UnregisterClient() {
	unregisterStartTime := time.Now()
	count := atomic.AddInt32(&c.ClientCount, -1)

	c.logger.InfoEvent().
		Str("component", "StreamCoordinator").
		Str("operation", "unregister_client").
		Str("stream_id", c.streamID).
		Int("remaining_clients", int(count)).
		Msg("Client unregistered")

	if count == 0 {
		cleanupStartTime := time.Now()
		c.logger.InfoEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "cleanup_resources").
			Str("stream_id", c.streamID).
			Msg("Last client unregistered, cleaning up resources")

		atomic.StoreInt32(&c.state, stateDraining)

		// Signal the writer to shut down.
		signalStartTime := time.Now()
		select {
		case c.WriterChan <- struct{}{}:
			signalDuration := time.Since(signalStartTime)
			c.logger.DebugEvent().
				Str("component", "StreamCoordinator").
				Str("operation", "signal_writer_shutdown").
				Str("stream_id", c.streamID).
				Dur("signal_duration", signalDuration).
				Msg("Sent shutdown signal to writer")
		default:
			signalDuration := time.Since(signalStartTime)
			c.logger.DebugEvent().
				Str("component", "StreamCoordinator").
				Str("operation", "signal_writer_shutdown").
				Str("stream_id", c.streamID).
				Dur("signal_duration", signalDuration).
				Msg("Writer channel already has shutdown signal")
		}

		c.WriterRespHeader.Store(nil)

		// Measure buffer cleanup time
		bufferCleanupStartTime := time.Now()
		c.ClearBuffer()
		bufferCleanupDuration := time.Since(bufferCleanupStartTime)

		c.notifySubscribers()

		cleanupDuration := time.Since(cleanupStartTime)
		unregisterDuration := time.Since(unregisterStartTime)

		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "cleanup_complete").
			Str("stream_id", c.streamID).
			Dur("buffer_cleanup_duration", bufferCleanupDuration).
			Dur("cleanup_duration", cleanupDuration).
			Dur("unregister_duration", unregisterDuration).
			Msg("Resource cleanup completed")
	}
}

func (c *StreamCoordinator) HasClient() bool {
	return atomic.LoadInt32(&c.ClientCount) > 0
}

// shouldTimeout checks if the time since the last successful read exceeds the timeout.
func (c *StreamCoordinator) shouldTimeout(lastSuccess time.Time, timeout time.Duration) bool {
	shouldTimeout := c.config.TimeoutSeconds > 0 && time.Since(lastSuccess) >= timeout
	if shouldTimeout {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "should_timeout").
			Dur("time_since_last_success", time.Since(lastSuccess)).
			Msg("Stream timed out")
	}
	return shouldTimeout
}

// shouldRetry indicates whether the writer should retry reading on error.
func (c *StreamCoordinator) shouldRetry(timeout time.Duration) bool {
	return c.config.TimeoutSeconds == 0 || timeout > 0
}

// Write performs a zero‑copy write via a buffer swap.
// Regardless of success or failure, the provided chunk is immediately reset,
// transferring full buffer ownership to the coordinator and returning the old
// buffer back to the pool. This prevents any leaks or accidental reuse.
func (c *StreamCoordinator) Write(chunk *ChunkData) bool {
	writeStartTime := time.Now()

	if chunk == nil {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "buffer_write").
			Str("stream_id", c.streamID).
			Dur("write_duration", time.Since(writeStartTime)).
			Msg("Write: Received nil chunk")
		return false
	}

	lockStartTime := time.Now()
	c.Mu.Lock()
	lockDuration := time.Since(lockStartTime)

	// If the stream isn't active, we still Must consume (reset) the chunk.
	if atomic.LoadInt32(&c.state) != stateActive {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "buffer_write").
			Str("stream_id", c.streamID).
			Int("state", int(atomic.LoadInt32(&c.state))).
			Dur("lock_duration", lockDuration).
			Dur("write_duration", time.Since(writeStartTime)).
			Msg("Write: Stream not active")
		c.Mu.Unlock()
		chunk.Reset()
		return false
	}

	current, ok := c.Buffer.Value.(*ChunkData)
	if !ok || current == nil {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "buffer_write").
			Str("stream_id", c.streamID).
			Dur("lock_duration", lockDuration).
			Dur("write_duration", time.Since(writeStartTime)).
			Msg("Write: Current buffer position is nil")
		c.Mu.Unlock()
		chunk.Reset()
		return false
	}

	// Increment and assign a sequence number.
	current.seq = atomic.AddInt64(&c.writeSeq, 1)

	// Measure buffer swap time
	swapStartTime := time.Now()
	// Perform the swap:
	// - The ring's current chunk now receives the data from the caller's chunk.
	// - The caller's chunk is given the ring's old buffer.
	oldBuffer := current.Buffer
	current.Buffer = chunk.Buffer
	chunk.Buffer = oldBuffer

	current.Error = chunk.Error
	current.Status = chunk.Status
	current.Timestamp = chunk.Timestamp

	bytesWritten := 0
	if current.Buffer != nil {
		bytesWritten = current.Buffer.Len()
	}

	c.Buffer = c.Buffer.Next()
	swapDuration := time.Since(swapStartTime)

	c.logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("operation", "buffer_write").
		Str("stream_id", c.streamID).
		Int64("sequence", current.seq).
		Int("bytes_written", bytesWritten).
		Int("client_count", int(atomic.LoadInt32(&c.ClientCount))).
		Dur("lock_duration", lockDuration).
		Dur("swap_duration", swapDuration).
		Dur("write_duration", time.Since(writeStartTime)).
		Msg("Buffer write completed")

	// Mark error state if needed.
	if current.Error != nil || current.Status != 0 {
		if c.LastError.Load() == nil {
			c.LastError.Store(current)
		}
		atomic.StoreInt32(&c.state, stateDraining)
		c.logger.ErrorEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "buffer_write_error").
			Str("stream_id", c.streamID).
			Int64("sequence", current.seq).
			Int("status_code", current.Status).
			Dur("write_duration", time.Since(writeStartTime)).
			Err(current.Error).
			Msg("Write: Setting error state")
	}
	c.Mu.Unlock()

	// Measure notification time
	notifyStartTime := time.Now()
	c.notifySubscribers()
	notifyDuration := time.Since(notifyStartTime)

	// Enforce the new ownership rule:
	// Immediately reset the provided chunk so that its swapped-out buffer is
	// returned to the pool and the caller does not continue using stale data.
	resetStartTime := time.Now()
	chunk.Reset()
	resetDuration := time.Since(resetStartTime)

	totalWriteDuration := time.Since(writeStartTime)

	if bytesWritten > 0 {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "buffer_write_complete").
			Str("stream_id", c.streamID).
			Int("bytes_written", bytesWritten).
			Dur("notify_duration", notifyDuration).
			Dur("reset_duration", resetDuration).
			Dur("total_write_duration", totalWriteDuration).
			Msg("Buffer write operation completed")
	}

	return true
}

// ReadChunks retrieves chunks from the ring for a client, given a starting position.
func (c *StreamCoordinator) ReadChunks(fromPosition *ring.Ring) (
	[]*ChunkData, *ChunkData, *ring.Ring,
) {
	c.Mu.RLock()
	if fromPosition == nil {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("stream_id", c.streamID).
			Msg("ReadChunks: fromPosition is nil, using current buffer")
		fromPosition = c.Buffer
	}
	// Check if the client's pointer is too far behind.
	if cd, ok := fromPosition.Value.(*ChunkData); ok && cd != nil {
		currentWriteSeq := atomic.LoadInt64(&c.writeSeq)
		minSeq := currentWriteSeq - int64(c.config.SharedBufferSize)
		if cd.seq < minSeq {
			c.logger.DebugEvent().
				Str("component", "StreamCoordinator").
				Str("stream_id", c.streamID).
				Int64("client_seq", cd.seq).
				Int64("min_seq", minSeq).
				Int64("current_seq", currentWriteSeq).
				Msg("ReadChunks: Client pointer is stale; resetting to the latest chunk")
			fromPosition = c.Buffer
		}
	}

	// Wait if the client has caught up with the writer and the stream is active.
	for fromPosition == c.Buffer && atomic.LoadInt32(&c.state) == stateActive {
		c.Mu.RUnlock()
		ch := c.subscribe()
		<-ch
		c.Mu.RLock()
	}

	chunks := make([]*ChunkData, 0, c.config.SharedBufferSize)
	current := fromPosition
	var errorFound bool
	var errorChunk *ChunkData
	totalBytes := 0

	// Iterate until we reach the writer's current position.
	for current != c.Buffer {
		if chunk, ok := current.Value.(*ChunkData); ok && chunk != nil {
			if chunk.Buffer != nil && chunk.Buffer.Len() > 0 {
				newChunk := &ChunkData{
					Buffer:    bytebufferpool.Get(),
					Timestamp: chunk.Timestamp,
				}
				_, _ = newChunk.Buffer.Write(chunk.Buffer.Bytes())
				chunks = append(chunks, newChunk)
				totalBytes += chunk.Buffer.Len()
			}
			if chunk.Error != nil || chunk.Status != 0 {
				errorFound = true
				errorChunk = &ChunkData{
					Buffer:    nil,
					Error:     chunk.Error,
					Status:    chunk.Status,
					Timestamp: chunk.Timestamp,
				}
			}
		}
		current = current.Next()
		if current == fromPosition {
			break
		}
	}
	c.Mu.RUnlock()

	c.logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("stream_id", c.streamID).
		Int("chunks_read", len(chunks)).
		Int("total_bytes", totalBytes).
		Bool("error_found", errorFound).
		Int("client_count", int(atomic.LoadInt32(&c.ClientCount))).
		Msg("Buffer read completed")

	if errorFound && errorChunk != nil {
		return chunks, errorChunk, current
	}

	if lastErr := c.LastError.Load(); lastErr != nil {
		if errChunk, ok := lastErr.(*ChunkData); ok && errChunk != nil {
			return chunks, errChunk, current
		}
	}

	return chunks, nil, current
}

// ClearBuffer resets every chunk in the ring.
func (c *StreamCoordinator) ClearBuffer() {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	current := c.Buffer
	for i := 0; i < c.config.SharedBufferSize; i++ {
		if chunk, ok := current.Value.(*ChunkData); ok {
			chunk.Reset()
		}
		current = current.Next()
	}
}

// getTimeoutDuration returns the streaming timeout duration.
func (c *StreamCoordinator) getTimeoutDuration() time.Duration {
	if c.config.TimeoutSeconds == 0 {
		return time.Minute
	}
	return time.Duration(c.config.TimeoutSeconds) * time.Second
}

// writeError writes an error chunk to the stream, consuming the chunk.
func (c *StreamCoordinator) writeError(err error, status int) {
	chunk := newChunkData()
	if chunk == nil {
		c.logger.DebugEvent().
			Str("component", "StreamCoordinator").
			Str("operation", "write_error").
			Msg("Failed to create new chunk")
		return
	}
	chunk.Error = err
	chunk.Status = status
	chunk.Timestamp = time.Now()

	if !c.Write(chunk) {
		chunk.Reset()
	}
	atomic.StoreInt32(&c.state, stateClosed)
}

func (c *StreamCoordinator) readAndWriteStream(
	ctx context.Context,
	body io.ReadCloser,
	processChunk func([]byte) error,
) error {
	startTime := time.Now()
	buffer := make([]byte, c.config.ChunkSize)
	timeout := c.getTimeoutDuration()
	backoff := proxy.NewBackoffStrategy(c.config.InitialBackoff,
		time.Duration(c.config.TimeoutSeconds-1)*time.Second)

	lastSuccess := time.Now()
	lastErr := time.Now()
	zeroReads := 0
	totalBytesRead := 0
	readCount := 0

	c.logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("stream_id", c.streamID).
		Int("chunk_size", c.config.ChunkSize).
		Dur("timeout", timeout).
		Msg("Starting stream read and write loop")

	for atomic.LoadInt32(&c.state) == stateActive {
		select {
		case <-ctx.Done():
			c.logger.DebugEvent().
				Str("component", "StreamCoordinator").
				Str("stream_id", c.streamID).
				Dur("duration", time.Since(startTime)).
				Int("total_bytes_read", totalBytesRead).
				Int("read_count", readCount).
				Msg("Stream context cancelled")
			return ctx.Err()
		default:
			if c.shouldTimeout(lastSuccess, timeout) {
				c.logger.ErrorEvent().
					Str("component", "StreamCoordinator").
					Str("stream_id", c.streamID).
					Dur("duration", time.Since(startTime)).
					Dur("time_since_success", time.Since(lastSuccess)).
					Dur("timeout", timeout).
					Int("total_bytes_read", totalBytesRead).
					Msg("Stream timeout: no new segments")
				return fmt.Errorf("stream timeout: no new segments")
			}

			n, err := body.Read(buffer)
			readCount++

			if n > 0 {
				totalBytesRead += n
				c.logger.DebugEvent().
					Str("component", "StreamCoordinator").
					Str("stream_id", c.streamID).
					Int("bytes_read", n).
					Int("total_bytes_read", totalBytesRead).
					Int("read_count", readCount).
					Msg("Stream data read")
			}

			if n == 0 {
				zeroReads++
				if zeroReads > 10 {
					c.logger.DebugEvent().
						Str("component", "StreamCoordinator").
						Str("stream_id", c.streamID).
						Int("zero_reads", zeroReads).
						Dur("duration", time.Since(startTime)).
						Msg("Too many zero reads, ending stream")
					return io.EOF
				}
				time.Sleep(10 * time.Millisecond)
				continue
			}
			lastSuccess = time.Now()
			zeroReads = 0

			if err == io.EOF && n > 0 {
				if err = processChunk(buffer[:n]); err != nil {
					return err
				}
				c.logger.DebugEvent().
					Str("component", "StreamCoordinator").
					Str("stream_id", c.streamID).
					Dur("duration", time.Since(startTime)).
					Int("total_bytes_read", totalBytesRead).
					Msg("Stream ended with EOF after final chunk")
				return io.EOF
			}

			if err != nil {
				if c.shouldRetry(timeout) {
					c.logger.DebugEvent().
						Str("component", "StreamCoordinator").
						Str("stream_id", c.streamID).
						Err(err).
						Msg("Stream read error, retrying with backoff")
					backoff.Sleep(ctx)
					lastErr = time.Now()
					continue
				}
				c.logger.ErrorEvent().
					Str("component", "StreamCoordinator").
					Str("stream_id", c.streamID).
					Err(err).
					Dur("duration", time.Since(startTime)).
					Int("total_bytes_read", totalBytesRead).
					Msg("Stream read error, not retrying")
				return err
			}

			if err = processChunk(buffer[:n]); err != nil {
				c.logger.ErrorEvent().
					Str("component", "StreamCoordinator").
					Str("stream_id", c.streamID).
					Err(err).
					Int("chunk_size", n).
					Msg("Error processing chunk")
				return err
			}

			// Reset the backoff if at least one second has passed.
			if time.Since(lastErr) >= time.Second {
				backoff.Reset()
				lastErr = time.Now()
			}
		}
	}

	c.logger.DebugEvent().
		Str("component", "StreamCoordinator").
		Str("stream_id", c.streamID).
		Dur("duration", time.Since(startTime)).
		Int("total_bytes_read", totalBytesRead).
		Int("read_count", readCount).
		Msg("Stream read and write loop ended")
	return nil
}
