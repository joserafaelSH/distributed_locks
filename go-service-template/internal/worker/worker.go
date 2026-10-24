package worker

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/config"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/infra/cache"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/infra/postgres"
	"github.com/joserafaelSH/golang-software-engineer/go-service-template/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer = otel.Tracer("worker")
	meter  = otel.Meter("worker")
)

// workerMetrics holds the OTel instruments recorded across the poll/lock/job
// lifecycle. Created once per WorkerPoller so the exported series have a
// stable identity.
type workerMetrics struct {
	jobsFetched   metric.Int64Counter
	jobsClaimed   metric.Int64Counter
	jobsProcessed metric.Int64Counter
	jobDuration   metric.Float64Histogram
	pollDuration  metric.Float64Histogram
	lockAttempts  metric.Int64Counter
}

func newWorkerMetrics() (*workerMetrics, error) {
	jobsFetched, err := meter.Int64Counter("worker.jobs_fetched",
		metric.WithDescription("Jobs fetched from the pending queue"))
	if err != nil {
		return nil, fmt.Errorf("creating jobs_fetched counter: %w", err)
	}
	jobsClaimed, err := meter.Int64Counter("worker.jobs_claimed",
		metric.WithDescription("Jobs transitioned from pending to processing"))
	if err != nil {
		return nil, fmt.Errorf("creating jobs_claimed counter: %w", err)
	}
	jobsProcessed, err := meter.Int64Counter("worker.jobs_processed",
		metric.WithDescription("Jobs that finished processing, by outcome"))
	if err != nil {
		return nil, fmt.Errorf("creating jobs_processed counter: %w", err)
	}
	jobDuration, err := meter.Float64Histogram("worker.job_duration_seconds",
		metric.WithDescription("Time spent processing and updating a single job"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("creating job_duration histogram: %w", err)
	}
	pollDuration, err := meter.Float64Histogram("worker.poll_duration_seconds",
		metric.WithDescription("Time spent in a full poll cycle: lock acquire, poll, release"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, fmt.Errorf("creating poll_duration histogram: %w", err)
	}
	lockAttempts, err := meter.Int64Counter("worker.lock_attempts",
		metric.WithDescription("Poll-lock acquisition attempts, by outcome"))
	if err != nil {
		return nil, fmt.Errorf("creating lock_attempts counter: %w", err)
	}

	return &workerMetrics{
		jobsFetched:   jobsFetched,
		jobsClaimed:   jobsClaimed,
		jobsProcessed: jobsProcessed,
		jobDuration:   jobDuration,
		pollDuration:  pollDuration,
		lockAttempts:  lockAttempts,
	}, nil
}

type WorkerPoller struct {
	db           *sql.DB
	redis        *redis.Client
	pollInterval time.Duration
	batchSize    int
	cfg          *config.Config
	log          zerolog.Logger
	metrics      *workerMetrics
	// ownerID identifies this poller instance as the holder of the
	// distributed poll lock, so it can tell its own lock apart from one
	// held by another worker.
	ownerID string
	// jobSem bounds how many jobs are processed concurrently across all
	// polls, so a drain loop can't stack up unbounded goroutines/DB
	// connections while earlier jobs are still in flight.
	jobSem chan struct{}
}

func NewPoller(db *sql.DB, redis *redis.Client, cfg *config.Config, log zerolog.Logger) (*WorkerPoller, error) {
	hostname := cfg.ServiceName
	m, err := newWorkerMetrics()
	if err != nil {
		return nil, err
	}

	return &WorkerPoller{
		db:           db,
		redis:        redis,
		pollInterval: cfg.Worker.PollInterval,
		batchSize:    cfg.Worker.BatchSize,
		cfg:          cfg,
		log:          log.With().Str("component", "worker").Logger(),
		metrics:      m,
		ownerID:      fmt.Sprintf("%s-%d", hostname, os.Getpid()),
		jobSem:       make(chan struct{}, cfg.Worker.MaxConcurrentJobs),
	}, nil
}

// pollOnce fetches up to batchSize pending jobs, claims them (pending ->
// processing) and dispatches each to a goroutine for processing. It returns
// once jobs are claimed and dispatched — it does not wait for them to finish.
func (p *WorkerPoller) pollOnce(ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "worker.poll")
	defer span.End()
	log := logger.FromContext(ctx)

	jobs, err := postgres.FetchPendingJobs(p.db, p.batchSize)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch pending jobs failed")
		log.Error().Err(err).Msg("failed to fetch pending jobs")
		return err
	}
	span.SetAttributes(attribute.Int("jobs.fetched", len(jobs)))
	p.metrics.jobsFetched.Add(ctx, int64(len(jobs)))
	log.Debug().Int("count", len(jobs)).Msg("fetched pending jobs")

	if len(jobs) == 0 {
		return nil
	}

	ids := make([]int64, len(jobs))
	for i, job := range jobs {
		ids[i] = job.ID
	}

	claimed, err := postgres.BulkUpdateJobStatus(p.db, ids, "processing")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "claim jobs failed")
		log.Error().Err(err).Msg("failed to claim jobs")
		return err
	}
	span.SetAttributes(attribute.Int64("jobs.claimed", claimed))
	p.metrics.jobsClaimed.Add(ctx, claimed)
	log.Info().Int64("count", claimed).Msg("claimed jobs for processing")

	for _, job := range jobs {
		p.jobSem <- struct{}{}
		go p.processAndUpdateJob(ctx, job)
	}

	return nil
}

// processAndUpdateJob runs one job to completion and writes back its final
// status. It always releases its jobSem slot, even on panic-free early
// returns, so the semaphore accurately reflects in-flight work.
func (p *WorkerPoller) processAndUpdateJob(ctx context.Context, job postgres.Job) {
	defer func() { <-p.jobSem }()

	ctx, span := tracer.Start(ctx, "worker.process_job", trace.WithAttributes(attribute.Int64("job.id", job.ID)))
	defer span.End()
	log := logger.FromContext(ctx).With().Int64("job_id", job.ID).Logger()

	start := time.Now()
	status := "done"
	if err := processJob(ctx, &job, p.cfg.Worker.JobTimeout, p.cfg.Worker.FailurePercent); err != nil {
		status = "pending"
		span.RecordError(err)
		span.SetStatus(codes.Error, "job processing failed")
		log.Warn().Err(err).Msg("job processing failed, requeueing")
	} else {
		log.Info().Msg("job processed successfully")
	}

	duration := time.Since(start).Seconds()
	outcomeAttr := metric.WithAttributes(attribute.String("outcome", status))
	p.metrics.jobDuration.Record(ctx, duration, outcomeAttr)
	p.metrics.jobsProcessed.Add(ctx, 1, outcomeAttr)
	span.SetAttributes(attribute.String("job.outcome", status))

	if _, err := postgres.UpdateJobStatus(p.db, job.ID, status); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "status update failed")
		log.Error().Err(err).Str("status", status).Msg("failed to update job status")
	}
}

// processJob simulates doing work on a job: it "processes" for up to timeout
// (or returns early if ctx is cancelled), then randomly fails with
// probability failurePercent (0-100) to exercise the retry/lock-release path.
func processJob(ctx context.Context, job *postgres.Job, timeout time.Duration, failurePercent float64) error {
	select {
	case <-time.After(timeout):
	case <-ctx.Done():
		return ctx.Err()
	}

	if rand.Float64()*100 < failurePercent {
		return fmt.Errorf("job %d: simulated failure", job.ID)
	}

	return nil
}

func (p *WorkerPoller) Run(ctx context.Context) {
	ctx = logger.WithLogger(ctx, p.log)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info().Msg("poller: shutting down")
			return
		case <-ticker.C:
			p.runPollCycle(ctx)
		}
	}
}

// runPollCycle covers one full tick: acquire the distributed poll lock, poll
// for jobs, then release the lock. Wrapped in its own span/duration metric so
// it shows up as one trace/data point per tick regardless of which branch
// (skip, error, success) it takes.
func (p *WorkerPoller) runPollCycle(ctx context.Context) {
	ctx, span := tracer.Start(ctx, "worker.poll_cycle")
	defer span.End()
	log := logger.FromContext(ctx)

	start := time.Now()
	defer func() {
		p.metrics.pollDuration.Record(ctx, time.Since(start).Seconds())
	}()

	const lockKey = "worker_poll_lock"

	acquired, err := p.acquireLock(ctx, lockKey)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "lock acquisition error")
		log.Error().Err(err).Msg("error during lock acquisition")
	

	if err := p.pollOnce(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "poll failed")
		log.Error().Err(err).Msg("error polling for jobs")
	}

	if err := p.releaseLock(ctx, lockKey); err != nil {
		span.RecordError(err)
		log.Error().Err(err).Msg("error releasing poll lock")
	}
}

// acquireLock implements the poll-lock hand-off:
//  1. Check if the lock exists.
//  2. If it exists, check whether the owner is this poller or another one.
//  3. If the owner is self, proceed (we already hold it).
//  4. If the owner is another worker, skip this poll.
//  5. If the lock doesn't exist, try to acquire it.
//  6. If acquired, proceed.
//  7. If not acquired (lost the race), skip this poll.
//
// Returns (true, nil) only when this poller may proceed to poll.
func (p *WorkerPoller) acquireLock(ctx context.Context, key string) (bool, error) {
	ctx, span := tracer.Start(ctx, "worker.acquire_lock")
	defer span.End()
	log := logger.FromContext(ctx)

	recordOutcome := func(outcome string) {
		span.SetAttributes(attribute.String("lock.outcome", outcome))
		p.metrics.lockAttempts.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
	}

	exists, err := cache.LockExists(ctx, p.redis, key)
	if err != nil {
		return false, err
	}

	if exists {
		owner, err := cache.LockHasOwner(ctx, p.redis, key)
		if err != nil {
			return false, err
		}
		if owner != p.ownerID {
			recordOutcome("held_by_other")
			span.SetAttributes(attribute.String("lock.owner", owner))
			log.Debug().Str("owner", owner).Msg("poll lock held by another worker")
			return false, nil
		}
		recordOutcome("already_held")
		log.Debug().Msg("poll lock already held by this worker")
		return true, nil
	}

	acquired, err := cache.SetLock(ctx, p.redis, key, p.ownerID, p.cfg.Worker.LockTTL)
	if err != nil {
		return false, err
	}
	if !acquired {
		recordOutcome("lost_race")
		log.Debug().Msg("lost race to acquire poll lock")
		return false, nil
	}

	recordOutcome("acquired")
	log.Debug().Msg("acquired poll lock")
	return true, nil
}

// releaseLock releases the poll lock only if this poller still owns it, so
// it never deletes a lock that expired and was re-acquired by another worker
// while this on	return
	}
	if !acquired {
		log.Debug().Msg("poll lock unavailable, skipping this tick")
		return
	}e was busy polling.
func (p *WorkerPoller) releaseLock(ctx context.Context, key string) error {
	ctx, span := tracer.Start(ctx, "worker.release_lock")
	defer span.End()
	log := logger.FromContext(ctx)

	owner, err := cache.LockHasOwner(ctx, p.redis, key)
	if err != nil {
		span.RecordError(err)
		return err
	}
	if owner != p.ownerID {
		span.SetAttributes(attribute.Bool("lock.released", false))
		log.Debug().Str("owner", owner).Msg("poll lock no longer owned by this worker, not releasing")
		return nil
	}

	if err := p.redis.Del(ctx, key).Err(); err != nil {
		span.RecordError(err)
		return err
	}
	span.SetAttributes(attribute.Bool("lock.released", true))
	log.Debug().Msg("released poll lock")
	return nil
}
