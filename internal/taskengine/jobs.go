package taskengine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const Queue = "task-solver"

type SolveArgs struct {
	JobID string `json:"job_id" river:"unique"`
}

func (SolveArgs) Kind() string { return "task_solve" }

type RiverJobs struct {
	mu     sync.RWMutex
	client *river.Client[pgx.Tx]
}

func NewRiverJobs() *RiverJobs { return &RiverJobs{} }

func (jobs *RiverJobs) SetClient(client *river.Client[pgx.Tx]) error {
	if jobs == nil || client == nil {
		return ErrInvalidInput
	}
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	if jobs.client != nil {
		return ErrConflict
	}
	jobs.client = client
	return nil
}

func (jobs *RiverJobs) EnqueueSolve(ctx context.Context, id Identifier) (int64, error) {
	if id.IsZero() {
		return 0, ErrInvalidInput
	}
	client, err := jobs.readyClient()
	if err != nil {
		return 0, err
	}
	result, err := client.Insert(ctx, SolveArgs{JobID: id.String()}, &river.InsertOpts{
		MaxAttempts: MaximumAttempts, Queue: Queue, UniqueOpts: river.UniqueOpts{ByArgs: true},
	})
	if err != nil {
		return 0, fmt.Errorf("enqueue task solve: %w", err)
	}
	if result == nil || result.Job == nil || result.Job.ID <= 0 {
		return 0, ErrInvalidState
	}
	return result.Job.ID, nil
}

func (jobs *RiverJobs) Cancel(ctx context.Context, jobID int64) error {
	if jobID <= 0 {
		return ErrInvalidInput
	}
	client, err := jobs.readyClient()
	if err != nil {
		return err
	}
	if _, err := client.JobCancel(ctx, jobID); err != nil && !errors.Is(err, river.ErrNotFound) {
		return fmt.Errorf("cancel task solve: %w", err)
	}
	return nil
}

func (jobs *RiverJobs) readyClient() (*river.Client[pgx.Tx], error) {
	if jobs == nil {
		return nil, ErrInvalidState
	}
	jobs.mu.RLock()
	defer jobs.mu.RUnlock()
	if jobs.client == nil {
		return nil, ErrInvalidState
	}
	return jobs.client, nil
}

func NewRiverClient(pool *pgxpool.Pool, workers *river.Workers, execute bool, timeout time.Duration) (*river.Client[pgx.Tx], error) {
	if pool == nil || timeout < time.Second || timeout > 10*time.Minute {
		return nil, ErrInvalidInput
	}
	config := &river.Config{JobTimeout: timeout + 10*time.Second, RescueStuckJobsAfter: timeout + time.Minute, SoftStopTimeout: 30 * time.Second}
	if execute {
		if workers == nil {
			return nil, ErrInvalidInput
		}
		config.Workers = workers
		config.Queues = map[string]river.QueueConfig{Queue: {MaxWorkers: 2}}
	} else {
		config.SkipUnknownJobCheck = true
	}
	client, err := river.NewClient(riverpgxv5.New(pool), config)
	if err != nil {
		return nil, fmt.Errorf("configure task queue: %w", err)
	}
	return client, nil
}

func NewRuntime(pool *pgxpool.Pool, query QueryGateway, interpreter Interpreter, options ServiceOptions, execute bool) (*Service, *river.Client[pgx.Tx], error) {
	if pool == nil || query == nil {
		return nil, nil, ErrInvalidInput
	}
	jobs := NewRiverJobs()
	var workers *river.Workers
	if execute {
		workers = river.NewWorkers()
	}
	service, err := NewService(NewPostgresStore(pool), jobs, query, interpreter, options)
	if err != nil {
		return nil, nil, err
	}
	if execute {
		river.AddWorker(workers, &solveWorker{service: service})
	}
	client, err := NewRiverClient(pool, workers, execute, service.timeout)
	if err != nil {
		return nil, nil, err
	}
	if err := jobs.SetClient(client); err != nil {
		return nil, nil, err
	}
	return service, client, nil
}

type solveWorker struct {
	river.WorkerDefaults[SolveArgs]
	service *Service
}

func (worker *solveWorker) Work(ctx context.Context, job *river.Job[SolveArgs]) error {
	id, err := queryIdentifier(job.Args.JobID)
	if err != nil {
		return river.JobCancel(err)
	}
	if err := worker.service.RunJob(ctx, id); err != nil {
		current, loadErr := worker.service.store.GetJobForWorker(context.WithoutCancel(ctx), id)
		if loadErr == nil && current.State == JobRunning && current.AttemptCount < MaximumAttempts && !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrInvalidSpec) {
			return err
		}
		return river.JobCancel(err)
	}
	return nil
}

func queryIdentifier(value string) (Identifier, error) {
	return IdentifierFromString(value)
}
