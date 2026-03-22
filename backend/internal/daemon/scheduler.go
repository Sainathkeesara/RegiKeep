package daemon

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/regikeep/rgk/internal/core"
	"github.com/regikeep/rgk/internal/registry"
	"github.com/regikeep/rgk/internal/store"
)

// Status represents the daemon's current state.
type Status string

const (
	StatusStopped Status = "stopped"
	StatusRunning Status = "running"
)

// Scheduler periodically runs keepalive for all enabled groups.
type Scheduler struct {
	db        *store.DB
	regMgr    *registry.Manager
	log       zerolog.Logger
	workers   int

	mu       sync.Mutex
	cancelFn context.CancelFunc
	status   atomic.Value // stores Status
	lastRun  atomic.Value // stores time.Time
	nextRun  atomic.Value // stores time.Time
}

// NewScheduler creates a Scheduler.
func NewScheduler(db *store.DB, reg *registry.Manager, workers int, log zerolog.Logger) *Scheduler {
	s := &Scheduler{
		db:      db,
		regMgr:  reg,
		log:     log,
		workers: workers,
	}
	s.status.Store(StatusStopped)
	return s
}

// Start launches the background keepalive loop. Idempotent.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status.Load().(Status) == StatusRunning {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel
	s.status.Store(StatusRunning)

	go s.run(ctx)
	s.log.Info().Msg("daemon scheduler started")
	return nil
}

// Stop cancels the background loop and waits for it to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status.Load().(Status) == StatusStopped {
		return
	}

	s.cancelFn()
	s.status.Store(StatusStopped)
	s.log.Info().Msg("daemon scheduler stopped")
}

// Status returns the current daemon state.
func (s *Scheduler) GetStatus() DaemonStatus {
	lastRun, _ := s.lastRun.Load().(time.Time)
	nextRun, _ := s.nextRun.Load().(time.Time)

	ds := DaemonStatus{
		Status:  string(s.status.Load().(Status)),
		Workers: s.workers,
	}
	if !lastRun.IsZero() {
		t := lastRun.Format(time.RFC3339)
		ds.LastRun = &t
	}
	if !nextRun.IsZero() {
		t := nextRun.Format(time.RFC3339)
		ds.NextRun = &t
	}
	return ds
}

// DaemonStatus is the API-facing status shape.
type DaemonStatus struct {
	Status  string  `json:"status"`
	Workers int     `json:"workers"`
	LastRun *string `json:"lastRun"`
	NextRun *string `json:"nextRun"`
}

func (s *Scheduler) run(ctx context.Context) {
	// Run once immediately, then on a ticker.
	s.runCycle(ctx)

	ticker := time.NewTicker(60 * time.Second) // check every 60s if any group is due
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runCycle(ctx)
		}
	}
}

func (s *Scheduler) runCycle(ctx context.Context) {
	groups, err := s.db.ListGroups()
	if err != nil {
		s.log.Error().Err(err).Msg("scheduler: failed to list groups")
		return
	}

	svc := core.NewKeepaliveService(s.db, s.regMgr, s.log)

	sem := make(chan struct{}, s.workers)
	var wg sync.WaitGroup

	for _, g := range groups {
		if !g.Enabled {
			continue
		}

		interval, err := parseDuration(g.Interval)
		if err != nil {
			s.log.Warn().Str("group", g.Name).Str("interval", g.Interval).Msg("invalid interval")
			continue
		}

		images, err := s.db.ListImages(store.ImageFilter{GroupID: g.ID})
		if err != nil {
			continue
		}

		for _, img := range images {
			if !isDue(img, interval) {
				continue
			}

			wg.Add(1)
			sem <- struct{}{}
			go func(img store.ImageRef, grp store.Group) {
				defer wg.Done()
				defer func() { <-sem }()

				if ctx.Err() != nil {
					return
				}

				_, err := svc.Run(core.KeepaliveRequest{
					ImageIDs: []string{img.ID},
					Strategy: grp.Strategy,
				})
				if err != nil {
					s.log.Error().Err(err).Str("image_id", img.ID).Msg("keepalive failed in scheduler")
				}
			}(img, g)
		}
	}

	wg.Wait()
	now := time.Now().UTC()
	s.lastRun.Store(now)
	s.nextRun.Store(now.Add(60 * time.Second))
}

func isDue(img store.ImageRef, interval time.Duration) bool {
	if img.LastKeepaliveAt == nil {
		return true
	}
	return time.Since(*img.LastKeepaliveAt) >= interval
}

// parseDuration parses intervals like "7d", "24h", "30m".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid interval %q", s)
	}
	unit := s[len(s)-1]
	value, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return 0, fmt.Errorf("invalid interval %q: %w", s, err)
	}
	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	case 'm':
		return time.Duration(value) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown unit %q in interval %q", unit, s)
	}
}
