package loadbalancer

import (
	"context"
	"fmt"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/proxy"
	"m3u-stream-merger/sourceproc"
	"m3u-stream-merger/store"
	"m3u-stream-merger/utils"
	"net/http"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

type LoadBalancerInstance struct {
	infoMu        sync.Mutex
	info          *sourceproc.StreamInfo
	Cm            *store.ConcurrencyManager
	config        *LBConfig
	httpClient    HTTPClient
	logger        logger.Logger
	indexProvider IndexProvider
	slugParser    SlugParser
	testedIndexes *xsync.MapOf[string, []string]
}

type LoadBalancerInstanceOption func(*LoadBalancerInstance)

func WithHTTPClient(client HTTPClient) LoadBalancerInstanceOption {
	return func(s *LoadBalancerInstance) {
		s.httpClient = client
	}
}

func WithLogger(logger logger.Logger) LoadBalancerInstanceOption {
	return func(s *LoadBalancerInstance) {
		s.logger = logger
	}
}

func WithIndexProvider(provider IndexProvider) LoadBalancerInstanceOption {
	return func(s *LoadBalancerInstance) {
		s.indexProvider = provider
	}
}

func WithSlugParser(parser SlugParser) LoadBalancerInstanceOption {
	return func(s *LoadBalancerInstance) {
		s.slugParser = parser
	}
}

func NewLoadBalancerInstance(
	cm *store.ConcurrencyManager,
	cfg *LBConfig,
	opts ...LoadBalancerInstanceOption,
) *LoadBalancerInstance {
	instance := &LoadBalancerInstance{
		Cm:            cm,
		config:        cfg,
		httpClient:    utils.HTTPClient,
		logger:        &logger.DefaultLogger{},
		indexProvider: &DefaultIndexProvider{},
		slugParser:    &DefaultSlugParser{},
		testedIndexes: xsync.NewMapOf[string, []string](),
	}

	for _, opt := range opts {
		opt(instance)
	}

	return instance
}

type LoadBalancerResult struct {
	Response *http.Response
	URL      string
	Index    string
	SubIndex string
}

func (instance *LoadBalancerInstance) GetStreamInfo() *sourceproc.StreamInfo {
	instance.infoMu.Lock()
	defer instance.infoMu.Unlock()
	return instance.info
}

func (instance *LoadBalancerInstance) SetStreamInfo(info *sourceproc.StreamInfo) {
	instance.infoMu.Lock()
	defer instance.infoMu.Unlock()
	instance.info = info
}

func (instance *LoadBalancerInstance) GetStreamId(req *http.Request) string {
	streamId := strings.Split(path.Base(req.URL.Path), ".")[0]
	if streamId == "" {
		return ""
	}
	streamId = strings.TrimPrefix(streamId, "/")

	return streamId
}

func (instance *LoadBalancerInstance) Balance(ctx context.Context, req *http.Request) (*LoadBalancerResult, error) {
	startTime := time.Now()
	
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if req == nil {
		return nil, fmt.Errorf("req cannot be nil")
	}
	if req.Method == "" {
		return nil, fmt.Errorf("req.Method cannot be empty")
	}
	if req.URL == nil {
		return nil, fmt.Errorf("req.URL cannot be empty")
	}

	// Create a logger with correlation ID
	requestLogger := instance.logger.WithCorrelationID(ctx)

	streamId := instance.GetStreamId(req)

	requestLogger.InfoEvent().
		Str("component", "LoadBalancerInstance").
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Str("client_ip", req.RemoteAddr).
		Str("stream_id", streamId).
		Msg("Starting load balancing process")

	err := instance.fetchBackendUrls(streamId, requestLogger)
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "LoadBalancerInstance").
			Str("stream_id", streamId).
			Dur("duration", time.Since(startTime)).
			Err(err).
			Msg("Error fetching backend URLs")
		return nil, fmt.Errorf("error fetching sources for: %s", streamId)
	}

	backoff := proxy.NewBackoffStrategy(time.Duration(instance.config.RetryWait)*time.Second, 0)

	for lap := 0; lap < instance.config.MaxRetries || instance.config.MaxRetries == 0; lap++ {
		requestLogger.DebugEvent().
			Str("component", "LoadBalancerInstance").
			Str("stream_id", streamId).
			Int("attempt_count", lap+1).
			Int("max_retries", instance.config.MaxRetries).
			Str("operation", "load_balance_attempt").
			Msg("Load balancer attempt")

		result, err := instance.tryAllStreams(ctx, req.Method, streamId, requestLogger)
		if err == nil {
			requestLogger.InfoEvent().
				Str("component", "LoadBalancerInstance").
				Str("stream_id", streamId).
				Str("selected_url", result.URL).
				Str("lb_index", result.Index).
				Str("lb_sub_index", result.SubIndex).
				Int("attempt_count", lap+1).
				Dur("duration", time.Since(startTime)).
				Msg("Load balancing completed successfully")
			return result, nil
		}
		
		requestLogger.DebugEvent().
			Str("component", "LoadBalancerInstance").
			Str("stream_id", streamId).
			Int("attempt_count", lap+1).
			Str("operation", "load_balance_attempt").
			Err(err).
			Msg("Load balancer attempt failed")

		if err == context.Canceled {
			requestLogger.WarnEvent().
				Str("component", "LoadBalancerInstance").
				Str("stream_id", streamId).
				Dur("duration", time.Since(startTime)).
				Msg("Load balancing cancelled by context")
			return nil, fmt.Errorf("cancelling load balancer")
		}

		instance.clearTested(streamId)

		select {
		case <-time.After(backoff.Next()):
		case <-ctx.Done():
			requestLogger.WarnEvent().
				Str("component", "LoadBalancerInstance").
				Str("stream_id", streamId).
				Dur("duration", time.Since(startTime)).
				Msg("Load balancing cancelled during backoff")
			return nil, fmt.Errorf("cancelling load balancer")
		}
	}

	requestLogger.ErrorEvent().
		Str("component", "LoadBalancerInstance").
		Str("stream_id", streamId).
		Int("attempt_count", instance.config.MaxRetries).
		Dur("duration", time.Since(startTime)).
		Msg("Load balancing exhausted all streams")
	return nil, fmt.Errorf("error fetching stream: exhausted all streams")
}

func (instance *LoadBalancerInstance) GetNumTestedIndexes(streamId string) int {
	streamTested, ok := instance.testedIndexes.Load(streamId)
	if !ok {
		return 0
	}
	return len(streamTested)
}

func (instance *LoadBalancerInstance) fetchBackendUrls(streamUrl string, requestLogger logger.Logger) error {
	fetchStartTime := time.Now()
	
	requestLogger.DebugEvent().
		Str("component", "LoadBalancerInstance").
		Str("operation", "fetch_backend_urls").
		Str("stream_url", streamUrl).
		Msg("Starting backend URL fetch")

	// Measure slug parsing time
	slugParseStartTime := time.Now()
	stream, err := instance.slugParser.GetStreamBySlug(streamUrl)
	slugParseDuration := time.Since(slugParseStartTime)
	
	if err != nil {
		requestLogger.ErrorEvent().
			Str("component", "LoadBalancerInstance").
			Str("operation", "parse_slug").
			Str("stream_url", streamUrl).
			Dur("slug_parse_duration", slugParseDuration).
			Dur("fetch_duration", time.Since(fetchStartTime)).
			Err(err).
			Msg("Failed to parse stream slug")
		return err
	}

	requestLogger.DebugEvent().
		Str("component", "LoadBalancerInstance").
		Str("operation", "parse_slug").
		Str("stream_url", streamUrl).
		Dur("slug_parse_duration", slugParseDuration).
		Msg("Successfully decoded slug")

	if stream.URLs == nil {
		stream.URLs = xsync.NewMapOf[string, map[string]string]()
	}
	
	// Measure URL validation time
	validationStartTime := time.Now()
	if stream.URLs.Size() == 0 {
		validationDuration := time.Since(validationStartTime)
		requestLogger.ErrorEvent().
			Str("component", "LoadBalancerInstance").
			Str("operation", "validate_urls").
			Str("stream_url", streamUrl).
			Dur("validation_duration", validationDuration).
			Dur("fetch_duration", time.Since(fetchStartTime)).
			Msg("Stream has no URLs configured")
		return fmt.Errorf("stream has no URLs configured")
	}

	// Validate that at least one index has URLs
	hasValidUrls := false
	urlCount := 0
	stream.URLs.Range(func(_ string, innerMap map[string]string) bool {
		urlCount += len(innerMap)
		if len(innerMap) > 0 {
			hasValidUrls = true
			return false
		}
		return true
	})
	validationDuration := time.Since(validationStartTime)
	
	if !hasValidUrls {
		requestLogger.ErrorEvent().
			Str("component", "LoadBalancerInstance").
			Str("operation", "validate_urls").
			Str("stream_url", streamUrl).
			Int("total_url_count", urlCount).
			Dur("validation_duration", validationDuration).
			Dur("fetch_duration", time.Since(fetchStartTime)).
			Msg("Stream has no valid URLs")
		return fmt.Errorf("stream has no valid URLs")
	}

	instance.SetStreamInfo(stream)
	
	fetchDuration := time.Since(fetchStartTime)
	requestLogger.InfoEvent().
		Str("component", "LoadBalancerInstance").
		Str("operation", "fetch_backend_urls").
		Str("stream_url", streamUrl).
		Int("index_count", stream.URLs.Size()).
		Int("total_url_count", urlCount).
		Dur("slug_parse_duration", slugParseDuration).
		Dur("validation_duration", validationDuration).
		Dur("fetch_duration", fetchDuration).
		Msg("Backend URLs fetched successfully")

	return nil
}

func (instance *LoadBalancerInstance) tryAllStreams(ctx context.Context, method string, streamId string, requestLogger logger.Logger) (*LoadBalancerResult, error) {
	requestLogger.InfoEvent().
		Str("component", "LoadBalancerInstance").
		Str("stream_id", streamId).
		Str("method", method).
		Msg("Trying all stream URLs")
		
	if instance.indexProvider == nil {
		return nil, fmt.Errorf("index provider cannot be nil")
	}
	m3uIndexes := instance.indexProvider.GetM3UIndexes()
	if len(m3uIndexes) == 0 {
		requestLogger.ErrorEvent().
			Str("component", "LoadBalancerInstance").
			Str("stream_id", streamId).
			Msg("No M3U indexes available")
		return nil, fmt.Errorf("no M3U indexes available")
	}

	requestLogger.DebugEvent().
		Str("component", "LoadBalancerInstance").
		Str("stream_id", streamId).
		Int("available_indexes", len(m3uIndexes)).
		Msg("Available M3U indexes for load balancing")

	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
		done := make(map[string]bool)
		initialCount := len(m3uIndexes)

		for len(done) < initialCount {
			sort.Slice(m3uIndexes, func(i, j int) bool {
				return instance.Cm.ConcurrencyPriorityValue(m3uIndexes[i]) > instance.Cm.ConcurrencyPriorityValue(m3uIndexes[j])
			})

			var index string
			for _, idx := range m3uIndexes {
				if !done[idx] {
					index = idx
					break
				}
			}

			done[index] = true

			requestLogger.DebugEvent().
				Str("component", "LoadBalancerInstance").
				Str("stream_id", streamId).
				Str("index", index).
				Int("concurrency_priority", instance.Cm.ConcurrencyPriorityValue(index)).
				Msg("Trying M3U index")

			innerMap, ok := instance.GetStreamInfo().URLs.Load(index)
			if !ok {
				requestLogger.ErrorEvent().
					Str("component", "LoadBalancerInstance").
					Str("stream_id", streamId).
					Str("index", index).
					Str("title", instance.GetStreamInfo().Title).
					Msg("Channel not found from M3U")
				continue
			}

			requestLogger.DebugEvent().
				Str("component", "LoadBalancerInstance").
				Str("stream_id", streamId).
				Str("index", index).
				Int("url_count", len(innerMap)).
				Msg("Found URLs in M3U index")

			result, err := instance.tryStreamUrls(method, streamId, index, innerMap, requestLogger)
			if err == nil {
				requestLogger.InfoEvent().
					Str("component", "LoadBalancerInstance").
					Str("stream_id", streamId).
					Str("selected_index", index).
					Str("selected_url", result.URL).
					Msg("Successfully selected stream URL")
				return result, nil
			}

			requestLogger.DebugEvent().
				Str("component", "LoadBalancerInstance").
				Str("stream_id", streamId).
				Str("index", index).
				Err(err).
				Msg("Failed to get stream from index")

			select {
			case <-ctx.Done():
				return nil, context.Canceled
			default:
				continue
			}
		}
	}
	requestLogger.WarnEvent().
		Str("component", "LoadBalancerInstance").
		Str("stream_id", streamId).
		Int("tried_indexes", len(m3uIndexes)).
		Msg("No available streams found")
	return nil, fmt.Errorf("no available streams")
}

func (instance *LoadBalancerInstance) tryStreamUrls(
	method string,
	streamId string,
	index string,
	urls map[string]string,
	requestLogger logger.Logger,
) (*LoadBalancerResult, error) {
	tryUrlsStartTime := time.Now()
	
	if instance.httpClient == nil {
		return nil, fmt.Errorf("HTTP client cannot be nil")
	}

	requestLogger.DebugEvent().
		Str("component", "LoadBalancerInstance").
		Str("operation", "try_stream_urls").
		Str("stream_id", streamId).
		Str("index", index).
		Int("url_count", len(urls)).
		Msg("Starting to try stream URLs")

	urlCount := 0
	for _, subIndex := range sourceproc.SortStreamSubUrls(urls) {
		urlTestStartTime := time.Now()
		urlCount++
		
		fileContent, ok := urls[subIndex]
		if !ok {
			continue
		}

		url := fileContent
		fileContentSplit := strings.SplitN(fileContent, ":::", 2)
		if len(fileContentSplit) == 2 {
			url = fileContentSplit[1]
		}

		id := index + "|" + subIndex
		
		// Measure tested check time
		testedCheckStartTime := time.Now()
		var alreadyTested bool
		streamTested, ok := instance.testedIndexes.Load(streamId)
		if ok {
			alreadyTested = slices.Contains(streamTested, index+"|"+subIndex)
		}
		testedCheckDuration := time.Since(testedCheckStartTime)

		if alreadyTested {
			requestLogger.DebugEvent().
				Str("component", "LoadBalancerInstance").
				Str("operation", "skip_tested_url").
				Str("stream_id", streamId).
				Str("index", index).
				Str("sub_index", subIndex).
				Str("url", url).
				Dur("tested_check_duration", testedCheckDuration).
				Dur("url_test_duration", time.Since(urlTestStartTime)).
				Msg("Skipping previously tested stream")
			continue
		}

		// Measure concurrency check time
		concurrencyCheckStartTime := time.Now()
		if instance.Cm.CheckConcurrency(index) {
			concurrencyCheckDuration := time.Since(concurrencyCheckStartTime)
			requestLogger.DebugEvent().
				Str("component", "LoadBalancerInstance").
				Str("operation", "skip_concurrency_limit").
				Str("stream_id", streamId).
				Str("index", index).
				Str("url", url).
				Dur("concurrency_check_duration", concurrencyCheckDuration).
				Dur("url_test_duration", time.Since(urlTestStartTime)).
				Msg("Concurrency limit reached for index")
			continue
		}
		concurrencyCheckDuration := time.Since(concurrencyCheckStartTime)

		requestLogger.DebugEvent().
			Str("component", "LoadBalancerInstance").
			Str("operation", "test_stream_url").
			Str("stream_id", streamId).
			Str("index", index).
			Str("sub_index", subIndex).
			Str("method", method).
			Str("url", url).
			Msg("Attempting to fetch stream")

		// Measure request creation time
		requestCreateStartTime := time.Now()
		req, err := http.NewRequest(method, url, nil)
		requestCreateDuration := time.Since(requestCreateStartTime)
		
		if err != nil {
			requestLogger.ErrorEvent().
				Str("component", "LoadBalancerInstance").
				Str("operation", "create_http_request").
				Str("stream_id", streamId).
				Str("index", index).
				Str("sub_index", subIndex).
				Str("method", method).
				Str("url", url).
				Dur("request_create_duration", requestCreateDuration).
				Dur("url_test_duration", time.Since(urlTestStartTime)).
				Err(err).
				Msg("Error creating request")
			instance.markTested(streamId, id)
			continue
		}

		// Measure HTTP request time
		httpRequestStartTime := time.Now()
		resp, err := instance.httpClient.Do(req)
		httpRequestDuration := time.Since(httpRequestStartTime)
		
		if err != nil {
			requestLogger.ErrorEvent().
				Str("component", "LoadBalancerInstance").
				Str("operation", "http_request").
				Str("stream_id", streamId).
				Str("index", index).
				Str("sub_index", subIndex).
				Str("method", method).
				Str("url", url).
				Dur("http_request_duration", httpRequestDuration).
				Dur("request_create_duration", requestCreateDuration).
				Dur("concurrency_check_duration", concurrencyCheckDuration).
				Dur("tested_check_duration", testedCheckDuration).
				Dur("url_test_duration", time.Since(urlTestStartTime)).
				Err(err).
				Msg("Error fetching stream")
			instance.markTested(streamId, id)
			continue
		}

		if resp == nil {
			requestLogger.ErrorEvent().
				Str("component", "LoadBalancerInstance").
				Str("operation", "http_request").
				Str("stream_id", streamId).
				Str("index", index).
				Str("sub_index", subIndex).
				Str("url", url).
				Dur("http_request_duration", httpRequestDuration).
				Dur("url_test_duration", time.Since(urlTestStartTime)).
				Msg("Received nil response from HTTP client")
			instance.markTested(streamId, id)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			requestLogger.ErrorEvent().
				Str("component", "LoadBalancerInstance").
				Str("operation", "http_request").
				Str("stream_id", streamId).
				Str("index", index).
				Str("sub_index", subIndex).
				Int("status_code", resp.StatusCode).
				Str("method", method).
				Str("url", url).
				Dur("http_request_duration", httpRequestDuration).
				Dur("url_test_duration", time.Since(urlTestStartTime)).
				Msg("Non-200 status code received")
			instance.markTested(streamId, id)
			continue
		}

		urlTestDuration := time.Since(urlTestStartTime)
		tryUrlsDuration := time.Since(tryUrlsStartTime)
		
		requestLogger.InfoEvent().
			Str("component", "LoadBalancerInstance").
			Str("operation", "stream_url_success").
			Str("stream_id", streamId).
			Str("index", index).
			Str("sub_index", subIndex).
			Str("method", method).
			Str("url", url).
			Int("status_code", resp.StatusCode).
			Int("urls_tested", urlCount).
			Dur("http_request_duration", httpRequestDuration).
			Dur("request_create_duration", requestCreateDuration).
			Dur("concurrency_check_duration", concurrencyCheckDuration).
			Dur("tested_check_duration", testedCheckDuration).
			Dur("url_test_duration", urlTestDuration).
			Dur("try_urls_duration", tryUrlsDuration).
			Msg("Successfully fetched stream")

		return &LoadBalancerResult{
			Response: resp,
			URL:      url,
			Index:    index,
			SubIndex: subIndex,
		}, nil
	}

	tryUrlsDuration := time.Since(tryUrlsStartTime)
	requestLogger.WarnEvent().
		Str("component", "LoadBalancerInstance").
		Str("operation", "try_stream_urls").
		Str("stream_id", streamId).
		Str("index", index).
		Int("urls_tested", urlCount).
		Dur("try_urls_duration", tryUrlsDuration).
		Msg("All URLs failed for index")

	return nil, fmt.Errorf("all urls failed")
}

func (instance *LoadBalancerInstance) markTested(streamId string, id string) {
	instance.testedIndexes.Compute(streamId, func(val []string, _ bool) (newValue []string, delete bool) {
		val = append(val, id)
		return val, false
	})
}

func (instance *LoadBalancerInstance) clearTested(streamId string) {
	instance.testedIndexes.Delete(streamId)
}
