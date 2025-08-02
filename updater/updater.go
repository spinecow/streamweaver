package updater

import (
	"context"
	"m3u-stream-merger/config"
	"m3u-stream-merger/handlers"
	"m3u-stream-merger/logger"
	"m3u-stream-merger/sourceproc"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Updater struct {
	sync.Mutex
	ctx        context.Context
	Cron       *cron.Cron
	logger     logger.Logger
	m3uHandler *handlers.M3UHTTPHandler
}

func Initialize(ctx context.Context, logger logger.Logger, m3uHandler *handlers.M3UHTTPHandler) (*Updater, error) {
	updateInstance := &Updater{
		ctx:        ctx,
		logger:     logger,
		m3uHandler: m3uHandler,
	}

	clearOnBoot := os.Getenv("CLEAR_ON_BOOT")
	if len(strings.TrimSpace(clearOnBoot)) == 0 {
		clearOnBoot = "false"
	}

	if clearOnBoot == "true" {
		updateInstance.logger.InfoEvent().
			Str("component", "Updater").
			Msg("CLEAR_ON_BOOT enabled. Clearing current cache")
		sourceproc.ClearProcessedM3Us()
	} else {
		latestM3u, err := config.GetLatestProcessedM3UPath()
		if err == nil {
			m3uHandler.SetProcessedPath(latestM3u)
		}
	}

	cronSched := os.Getenv("SYNC_CRON")
	if len(strings.TrimSpace(cronSched)) == 0 {
		updateInstance.logger.InfoEvent().
			Str("component", "Updater").
			Str("default_schedule", "0 0 * * *").
			Msg("SYNC_CRON not initialized. Defaulting to 12am every day")
		cronSched = "0 0 * * *"
	}

	c := cron.New()
	_, err := c.AddFunc(cronSched, func() {
		go updateInstance.UpdateSources(ctx)
	})
	if err != nil {
		updateInstance.logger.InfoEvent().
			Str("component", "Updater").
			Err(err).
			Msg("Error initializing background processes")
		return nil, err
	}
	c.Start()

	syncOnBoot := os.Getenv("SYNC_ON_BOOT")
	if len(strings.TrimSpace(syncOnBoot)) == 0 {
		syncOnBoot = "true"
	}

	if syncOnBoot == "true" {
		updateInstance.logger.InfoEvent().
			Str("component", "Updater").
			Msg("SYNC_ON_BOOT enabled. Starting initial M3U update")

		go updateInstance.UpdateSources(ctx)
	}

	updateInstance.Cron = c

	return updateInstance, nil
}

func (instance *Updater) UpdateSources(ctx context.Context) {
	updateStartTime := time.Now()
	
	// Ensure only one job is running at a time
	lockStartTime := time.Now()
	instance.Lock()
	lockDuration := time.Since(lockStartTime)
	defer instance.Unlock()

	instance.logger.InfoEvent().
		Str("component", "Updater").
		Str("operation", "update_sources").
		Dur("lock_duration", lockDuration).
		Msg("Starting source update process")

	// Measure processor creation time
	processorCreateStartTime := time.Now()
	processor := sourceproc.NewProcessor()
	processorCreateDuration := time.Since(processorCreateStartTime)
	
	select {
	case <-ctx.Done():
		instance.logger.WarnEvent().
			Str("component", "Updater").
			Str("operation", "update_sources").
			Dur("lock_duration", lockDuration).
			Dur("processor_create_duration", processorCreateDuration).
			Dur("update_duration", time.Since(updateStartTime)).
			Msg("Update cancelled by context")
		return
	default:
		instance.logger.InfoEvent().
			Str("component", "Updater").
			Str("operation", "update_sources").
			Dur("processor_create_duration", processorCreateDuration).
			Msg("Background process: Updating sources")

		instance.logger.InfoEvent().
			Str("component", "Updater").
			Str("operation", "build_m3u").
			Msg("Background process: Building merged M3U")
		
		// Measure environment check time
		envCheckStartTime := time.Now()
		if _, ok := os.LookupEnv("BASE_URL"); !ok {
			envCheckDuration := time.Since(envCheckStartTime)
			instance.logger.ErrorEvent().
				Str("component", "Updater").
				Str("operation", "check_environment").
				Dur("env_check_duration", envCheckDuration).
				Dur("update_duration", time.Since(updateStartTime)).
				Msg("BASE_URL is required for M3U processing to work.")
			return
		}
		envCheckDuration := time.Since(envCheckStartTime)
		
		// Measure M3U processing time
		processingStartTime := time.Now()
		err := processor.Run(ctx, nil)
		processingDuration := time.Since(processingStartTime)
		
		if err == nil {
			// Measure path setting time
			pathSetStartTime := time.Now()
			resultPath := processor.GetResultPath()
			instance.m3uHandler.SetProcessedPath(resultPath)
			pathSetDuration := time.Since(pathSetStartTime)
			
			updateDuration := time.Since(updateStartTime)
			instance.logger.InfoEvent().
				Str("component", "Updater").
				Str("operation", "update_sources").
				Str("result_path", resultPath).
				Int("processed_streams", processor.GetCount()).
				Dur("lock_duration", lockDuration).
				Dur("processor_create_duration", processorCreateDuration).
				Dur("env_check_duration", envCheckDuration).
				Dur("processing_duration", processingDuration).
				Dur("path_set_duration", pathSetDuration).
				Dur("update_duration", updateDuration).
				Msg("Source update completed successfully")
		} else {
			updateDuration := time.Since(updateStartTime)
			instance.logger.ErrorEvent().
				Str("component", "Updater").
				Str("operation", "update_sources").
				Dur("lock_duration", lockDuration).
				Dur("processor_create_duration", processorCreateDuration).
				Dur("env_check_duration", envCheckDuration).
				Dur("processing_duration", processingDuration).
				Dur("update_duration", updateDuration).
				Err(err).
				Msg("Source update failed")
		}
	}
}
