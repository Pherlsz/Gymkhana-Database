package googleforms

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

const Queue = "google_forms"

type SyncArgs struct {
	SyncRunID string `json:"sync_run_id" river:"unique"`
}

func (SyncArgs) Kind() string { return "google_forms_sync" }

type DueArgs struct {
	Window string `json:"window" river:"unique"`
}

func (DueArgs) Kind() string { return "google_forms_due" }

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

func (jobs *RiverJobs) EnqueueSync(ctx context.Context, id Identifier) (int64, error) {
	if id.IsZero() {
		return 0, ErrInvalidInput
	}
	return jobs.insert(ctx, SyncArgs{SyncRunID: id.String()}, 8)
}

func (jobs *RiverJobs) EnqueueDue(ctx context.Context, window time.Time) (int64, error) {
	if window.IsZero() {
		return 0, ErrInvalidInput
	}
	return jobs.insert(ctx, DueArgs{Window: window.UTC().Format(time.RFC3339)}, 5)
}

func (jobs *RiverJobs) insert(ctx context.Context, args river.JobArgs, attempts int) (int64, error) {
	client, err := jobs.readyClient()
	if err != nil {
		return 0, err
	}
	result, err := client.Insert(ctx, args, &river.InsertOpts{
		MaxAttempts: attempts,
		Queue:       Queue,
		UniqueOpts:  river.UniqueOpts{ByArgs: true},
	})
	if err != nil {
		return 0, fmt.Errorf("enqueue %s: %w", args.Kind(), err)
	}
	if result == nil || result.Job == nil || result.Job.ID <= 0 {
		return 0, fmt.Errorf("enqueue %s: %w", args.Kind(), ErrInvalidState)
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
		return fmt.Errorf("cancel google forms job: %w", err)
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

func NewRiverClient(pool *pgxpool.Pool, workers *river.Workers, execute bool) (*river.Client[pgx.Tx], error) {
	if pool == nil {
		return nil, ErrInvalidInput
	}
	config := &river.Config{
		JobTimeout:           10 * time.Minute,
		RescueStuckJobsAfter: 20 * time.Minute,
		SoftStopTimeout:      30 * time.Second,
	}
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
		return nil, fmt.Errorf("configure google forms queue: %w", err)
	}
	return client, nil
}

func RegisterRiverWorkers(workers *river.Workers, service *Service) error {
	if workers == nil || service == nil {
		return ErrInvalidInput
	}
	river.AddWorker(workers, &syncWorker{service: service})
	river.AddWorker(workers, &dueWorker{service: service})
	return nil
}

type syncWorker struct {
	river.WorkerDefaults[SyncArgs]
	service *Service
}

func (worker *syncWorker) Work(ctx context.Context, job *river.Job[SyncArgs]) error {
	id, err := ParseIdentifier(job.Args.SyncRunID)
	if err != nil {
		return river.JobCancel(err)
	}
	err = worker.service.Sync(ctx, id)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCancelled) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrInvalidInput) ||
		errors.Is(err, ErrInvalidState) || errors.Is(err, ErrSchemaDrift) || errors.Is(err, ErrNeedsReauth) ||
		errors.Is(err, ErrUnsupportedForm) || errors.Is(err, ErrProvider) {
		return river.JobCancel(err)
	}
	return err
}

type dueWorker struct {
	river.WorkerDefaults[DueArgs]
	service *Service
}

func (worker *dueWorker) Work(ctx context.Context, _ *river.Job[DueArgs]) error {
	_, err := worker.service.EnqueueDueSources(ctx)
	if errors.Is(err, ErrDisabled) {
		return nil
	}
	return err
}
