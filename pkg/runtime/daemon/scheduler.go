package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/robfig/cron/v3"
)

// Prometheus metrics for scheduler
var (
	jobExecutions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aixgo",
			Subsystem: "scheduler",
			Name:      "job_executions_total",
			Help:      "Total number of job executions",
		},
		[]string{"job", "status"},
	)

	jobDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aixgo",
			Subsystem: "scheduler",
			Name:      "job_duration_seconds",
			Help:      "Duration of job executions in seconds",
			Buckets:   []float64{.001, .005, .01, .05, .1, .5, 1, 5, 10, 30, 60, 300},
		},
		[]string{"job"},
	)

	jobLastRun = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "aixgo",
			Subsystem: "scheduler",
			Name:      "job_last_run_timestamp",
			Help:      "Timestamp of the last job run",
		},
		[]string{"job"},
	)

	schedulerRunning = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "aixgo",
			Subsystem: "scheduler",
			Name:      "running",
			Help:      "Whether the scheduler is running (1) or stopped (0)",
		},
	)

	jobsRegistered = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "aixgo",
			Subsystem: "scheduler",
			Name:      "jobs_registered",
			Help:      "Number of registered jobs",
		},
	)
)

// Job represents a scheduled job.
type Job struct {
	Name     string
	Schedule string // cron expression (e.g., "0 */6 * * *" for every 6 hours)
	Fn       func(ctx context.Context) error
}

// Scheduler manages periodic job execution using cron expressions.
type Scheduler struct {
	cron      *cron.Cron
	jobs      []Job
	logger    *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	isRunning bool
}

// NewScheduler creates a scheduler with the given logger.
func NewScheduler(logger *slog.Logger) *Scheduler {
	if logger == nil {
		logger = slog.Default()
	}

	// Create cron with seconds field support and recover from panics
	c := cron.New(
		cron.WithSeconds(),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),
		),
	)

	return &Scheduler{
		cron:   c,
		jobs:   make([]Job, 0),
		logger: logger,
	}
}

// AddJob adds a job with a cron expression.
// Cron expressions support 6 fields: second minute hour day month weekday
// Examples:
//   - "0 0 */6 * * *" - every 6 hours at minute 0, second 0
//   - "0 0 0 * * *"   - daily at midnight
//   - "0 */30 * * * *" - every 30 minutes
//   - "@every 1h"     - every hour (robfig/cron shorthand)
//   - "@daily"        - once a day at midnight
func (s *Scheduler) AddJob(name string, schedule string, fn func(ctx context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := Job{
		Name:     name,
		Schedule: schedule,
		Fn:       fn,
	}
	s.jobs = append(s.jobs, job)
	jobsRegistered.Set(float64(len(s.jobs)))

	s.logger.Info("job registered", "job", name, "schedule", schedule)
	return nil
}

// AddJobWithInterval adds a job that runs at a fixed interval.
// This is a convenience method that converts the interval to a cron expression.
func (s *Scheduler) AddJobWithInterval(name string, interval time.Duration, fn func(ctx context.Context) error) error {
	// Use robfig/cron's @every syntax for intervals
	schedule := "@every " + interval.String()
	return s.AddJob(name, schedule, fn)
}

// Name implements Task interface.
func (s *Scheduler) Name() string {
	return "scheduler"
}

// Start implements Task interface - starts the cron scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx) // #nosec G118 -- cancel released via defer s.cancel() on next line (belt-and-suspenders on top of Stop)
	// Ensure cancel is always invoked when Start returns, releasing
	// context resources even if Stop() is never called. Cancelling an
	// already-cancelled context is a no-op.
	defer s.cancel()
	s.isRunning = true
	s.mu.Unlock()

	// Register all jobs with the cron scheduler
	for _, job := range s.jobs {
		j := job // capture for closure
		_, err := s.cron.AddFunc(j.Schedule, func() {
			s.executeJob(s.ctx, j)
		})
		if err != nil {
			s.logger.Error("failed to add cron job", "job", j.Name, "schedule", j.Schedule, "error", err)
			continue
		}
		s.logger.Debug("cron job scheduled", "job", j.Name, "schedule", j.Schedule)
	}

	// Start the cron scheduler
	s.cron.Start()
	schedulerRunning.Set(1)
	s.logger.Info("scheduler started", "jobs", len(s.jobs))

	// Run all jobs immediately on startup
	for _, job := range s.jobs {
		j := job
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.executeJob(s.ctx, j)
		}()
	}

	// Wait for context cancellation
	<-s.ctx.Done()
	return nil
}

// Stop implements Task interface.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return nil
	}
	s.isRunning = false
	s.mu.Unlock()

	// Stop accepting new jobs
	cronCtx := s.cron.Stop()

	// Wait for running jobs to complete (with timeout from ctx)
	done := make(chan struct{})
	go func() {
		<-cronCtx.Done()
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("scheduler stopped gracefully")
	case <-ctx.Done():
		s.logger.Warn("scheduler stop timeout, some jobs may still be running")
	}

	if s.cancel != nil {
		s.cancel()
	}

	schedulerRunning.Set(0)
	return nil
}

func (s *Scheduler) executeJob(ctx context.Context, job Job) {
	// Check if context is cancelled
	if ctx.Err() != nil {
		return
	}

	s.logger.Info("running scheduled job", "job", job.Name)
	start := time.Now()

	// Record last run time
	jobLastRun.WithLabelValues(job.Name).SetToCurrentTime()

	// Execute the job
	err := job.Fn(ctx)
	duration := time.Since(start)

	// Record metrics
	jobDuration.WithLabelValues(job.Name).Observe(duration.Seconds())

	if err != nil {
		jobExecutions.WithLabelValues(job.Name, "error").Inc()
		s.logger.Error("job failed", "job", job.Name, "error", err, "duration", duration)
	} else {
		jobExecutions.WithLabelValues(job.Name, "success").Inc()
		s.logger.Info("job completed", "job", job.Name, "duration", duration)
	}
}

// GetNextRun returns the next scheduled run time for a job.
func (s *Scheduler) GetNextRun(jobName string) (time.Time, bool) {
	entries := s.cron.Entries()
	for _, entry := range entries {
		// Note: robfig/cron doesn't store job names, so we can't match by name
		// This is a limitation - for production, consider a custom wrapper
		_ = entry
	}
	return time.Time{}, false
}

// IsRunning returns whether the scheduler is currently running.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// JobCount returns the number of registered jobs.
func (s *Scheduler) JobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.jobs)
}
