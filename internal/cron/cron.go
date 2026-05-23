package cron

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Job represents a scheduled task.
type Job struct {
	Name     string
	Interval time.Duration
	NextRun  func(now time.Time) time.Time // optional, for specific times (e.g. 3am daily)
	Fn       func() error
}

// Scheduler runs periodic jobs.
type Scheduler struct {
	jobs   []*jobState
	logger *zap.Logger
	stopCh chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
	running bool
}

type jobState struct {
	Job
	lastRun  time.Time
	lastErr  error
	runCount int64
}

// New creates a new Scheduler.
func New(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

// AddJob registers a job with the scheduler.
func (s *Scheduler) AddJob(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, &jobState{Job: job})
}

// Start begins running all registered jobs.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	for i := range s.jobs {
		js := s.jobs[i]
		s.wg.Add(1)
		go s.runJob(js)
	}
	s.logger.Info("Cron scheduler started", zap.Int("jobs", len(s.jobs)))
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("Cron scheduler stopped")
}

func (s *Scheduler) runJob(js *jobState) {
	defer s.wg.Done()

	for {
		var nextRun time.Time
		now := time.Now()

		if js.NextRun != nil {
			nextRun = js.NextRun(now)
		} else {
			nextRun = now.Add(js.Interval)
			if !js.lastRun.IsZero() {
				nextRun = js.lastRun.Add(js.Interval)
				if nextRun.Before(now) {
					nextRun = now.Add(js.Interval)
				}
			}
		}

		waitDuration := nextRun.Sub(now)
		if waitDuration < 0 {
			waitDuration = time.Minute
		}

		timer := time.NewTimer(waitDuration)
		select {
		case <-s.stopCh:
			timer.Stop()
			return
		case <-timer.C:
			s.logger.Info("Running job", zap.String("name", js.Name))
			start := time.Now()
			err := js.Fn()
			elapsed := time.Since(start)

			s.mu.Lock()
			js.lastRun = time.Now()
			js.runCount++
			if err != nil {
				js.lastErr = err
				s.logger.Error("Job failed",
					zap.String("name", js.Name),
					zap.Duration("elapsed", elapsed),
					zap.Error(err),
				)
			} else {
				js.lastErr = nil
				s.logger.Info("Job completed",
					zap.String("name", js.Name),
					zap.Duration("elapsed", elapsed),
				)
			}
			s.mu.Unlock()
		}
	}
}

// Stats returns job execution statistics.
func (s *Scheduler) Stats() []JobStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	stats := make([]JobStats, len(s.jobs))
	for i, js := range s.jobs {
		lastErr := ""
		if js.lastErr != nil {
			lastErr = js.lastErr.Error()
		}
		stats[i] = JobStats{
			Name:     js.Name,
			LastRun:  js.lastRun,
			LastErr:  lastErr,
			RunCount: js.runCount,
		}
	}
	return stats
}

// JobStats holds execution statistics for a job.
type JobStats struct {
	Name     string    `json:"name"`
	LastRun  time.Time `json:"last_run"`
	LastErr  string    `json:"last_error,omitempty"`
	RunCount int64     `json:"run_count"`
}
