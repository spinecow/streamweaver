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
	// Ensure only one job is running at a time
	instance.Lock()
	defer instance.Unlock()

	processor := sourceproc.NewProcessor()
	select {
	case <-ctx.Done():
		return
	default:
		instance.logger.InfoEvent().
			Str("component", "Updater").
			Msg("Background process: Updating sources")

		instance.logger.InfoEvent().
			Str("component", "Updater").
			Msg("Background process: Building merged M3U")
		if _, ok := os.LookupEnv("BASE_URL"); !ok {
			instance.logger.ErrorEvent().
				Str("component", "Updater").
				Msg("BASE_URL is required for M3U processing to work.")
			return
		}
		if err := processor.Run(ctx, nil); err == nil {
			instance.m3uHandler.SetProcessedPath(processor.GetResultPath())
		}
	}
}
