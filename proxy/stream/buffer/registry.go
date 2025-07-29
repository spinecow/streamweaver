package buffer

import (
	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy/stream/config"
	"m3u-stream-merger/store"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

type StreamRegistry struct {
	coordinators  *xsync.MapOf[string, *StreamCoordinator]
	logger        logger.Logger
	config        *config.StreamConfig
	cleanupTicker *time.Ticker
	cm            *store.ConcurrencyManager
	done          chan struct{}

	Unrestrict bool
}

func NewStreamRegistry(config *config.StreamConfig, cm *store.ConcurrencyManager, logger logger.Logger, cleanupInterval time.Duration) *StreamRegistry {
	registry := &StreamRegistry{
		coordinators: xsync.NewMapOf[string, *StreamCoordinator](),
		logger:       logger,
		config:       config,
		cm:           cm,
		done:         make(chan struct{}),
	}

	if cleanupInterval > 0 {
		registry.cleanupTicker = time.NewTicker(cleanupInterval)
		go registry.runCleanup()
	}

	return registry
}

func (r *StreamRegistry) GetOrCreateCoordinator(streamID string, actualURL string) *StreamCoordinator {
	// First, try to find an existing coordinator by the actual URL
	if actualURL != "" {
		// Try to find an existing coordinator with the same actual URL
		var foundCoord *StreamCoordinator
		r.coordinators.Range(func(key string, coord *StreamCoordinator) bool {
			if coord.GetActualURL() == actualURL {
				foundCoord = coord
				return false // Stop iteration
			}
			return true // Continue iteration
		})
		
		if foundCoord != nil {
			r.logger.Debugf("Found existing coordinator for actual URL: %s", actualURL)
			return foundCoord
		}
	}

	// If no coordinator found by actual URL, use the streamID as the key
	coordId := streamID
	// if !r.Unrestrict {
	// 	streamInfo, err := sourceproc.DecodeSlug(streamID)
	// 	if err != nil {
	// 		r.logger.Logf("Invalid m3uID for GetOrCreateCoordinator from %s", streamID)
	// 		return nil
	// 	}

	// 	existingStreams := sourceproc.GetCurrentStreams()

	// 	if _, ok := existingStreams[streamInfo.Title]; !ok {
	// 		r.logger.Logf("Invalid m3uID for GetOrCreateCoordinator from %s", streamID)
	// 		return nil
	// 	}
	// 	coordId = streamInfo.Title
	// }

	if coord, ok := r.coordinators.Load(coordId); ok {
		// If we have an actual URL, set it on the existing coordinator
		if actualURL != "" {
			coord.SetActualURL(actualURL)
		}
		return coord
	}

	coord := NewStreamCoordinator(coordId, r.config, r.cm, r.logger)
	// If we have an actual URL, set it on the new coordinator
	if actualURL != "" {
		coord.SetActualURL(actualURL)
	}

	actual, loaded := r.coordinators.LoadOrStore(coordId, coord)
	if loaded {
		// If we have an actual URL, set it on the existing coordinator
		if actualURL != "" {
			actual.SetActualURL(actualURL)
		}
		return actual
	}

	return coord
}

func (r *StreamRegistry) RemoveCoordinator(coordId string) {
	r.coordinators.Delete(coordId)
}

// GetCoordinatorByActualURL returns a coordinator that matches the given actual URL
func (r *StreamRegistry) GetCoordinatorByActualURL(actualURL string) *StreamCoordinator {
	if actualURL == "" {
		return nil
	}
	
	var foundCoord *StreamCoordinator
	r.coordinators.Range(func(key string, coord *StreamCoordinator) bool {
		if coord.GetActualURL() == actualURL {
			foundCoord = coord
			return false // Stop iteration
		}
		return true // Continue iteration
	})
	
	return foundCoord
}

func (r *StreamRegistry) runCleanup() {
	for {
		select {
		case <-r.done:
			if r.cleanupTicker != nil {
				r.cleanupTicker.Stop()
			}
			return
		case <-r.cleanupTicker.C:
			r.cleanup()
		}
	}
}

func (r *StreamRegistry) cleanup() {
	r.coordinators.Range(func(key string, value *StreamCoordinator) bool {
		streamID := key
		coord := value

		if atomic.LoadInt32(&coord.ClientCount) == 0 {
			r.logger.Logf("Removing inactive coordinator for stream: %s", streamID)
			r.RemoveCoordinator(streamID)
		}
		return true
	})
}

func (r *StreamRegistry) Shutdown() {
	close(r.done)
	r.coordinators.Clear()
}
