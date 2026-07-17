package matching

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

const Queue = "matching"

type AnalysisArgs struct {
	AnalysisID string `json:"analysis_id" river:"unique"`
}

func (AnalysisArgs) Kind() string { return "matching_analyze_profiles" }

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

func (jobs *RiverJobs) EnqueueAnalysis(ctx context.Context, id Identifier) (int64, error) {
	if id.IsZero() {
		return 0, ErrInvalidInput
	}
	client, err := jobs.readyClient()
	if err != nil {
		return 0, err
	}
	result, err := client.Insert(ctx, AnalysisArgs{AnalysisID: id.String()}, &river.InsertOpts{
		MaxAttempts: 5,
		Queue:       Queue,
		UniqueOpts:  river.UniqueOpts{ByArgs: true},
	})
	if err != nil {
		return 0, fmt.Errorf("enqueue matching analysis: %w", err)
	}
	if result == nil || result.Job == nil || result.Job.ID <= 0 {
		return 0, fmt.Errorf("enqueue matching analysis: %w", ErrInvalidState)
	}
	return result.Job.ID, nil
}

func (jobs *RiverJobs) EnqueueAnalysisTx(ctx context.Context, tx pgx.Tx, id Identifier) (int64, error) {
	if tx == nil || id.IsZero() {
		return 0, ErrInvalidInput
	}
	client, err := jobs.readyClient()
	if err != nil {
		return 0, err
	}
	result, err := client.InsertTx(ctx, tx, AnalysisArgs{AnalysisID: id.String()}, &river.InsertOpts{
		MaxAttempts: 5,
		Queue:       Queue,
		UniqueOpts:  river.UniqueOpts{ByArgs: true},
	})
	if err != nil {
		return 0, fmt.Errorf("enqueue matching analysis transaction: %w", err)
	}
	if result == nil || result.Job == nil || result.Job.ID <= 0 {
		return 0, fmt.Errorf("enqueue matching analysis transaction: %w", ErrInvalidState)
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
		return fmt.Errorf("cancel matching analysis job: %w", err)
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
		JobTimeout:           2 * time.Minute,
		RescueStuckJobsAfter: 5 * time.Minute,
		SoftStopTimeout:      30 * time.Second,
	}
	if execute {
		if workers == nil {
			return nil, ErrInvalidInput
		}
		config.Workers = workers
		config.Queues = map[string]river.QueueConfig{Queue: {MaxWorkers: MaximumActiveAnalyses}}
	} else {
		config.SkipUnknownJobCheck = true
	}
	client, err := river.NewClient(riverpgxv5.New(pool), config)
	if err != nil {
		return nil, fmt.Errorf("configure matching queue: %w", err)
	}
	return client, nil
}

func RegisterRiverWorkers(workers *river.Workers, service *Service) error {
	if workers == nil || service == nil {
		return ErrInvalidInput
	}
	river.AddWorker(workers, &analysisWorker{service: service})
	return nil
}

func NewRuntime(pool *pgxpool.Pool, options ServiceOptions, execute bool) (*Service, *river.Client[pgx.Tx], error) {
	if pool == nil {
		return nil, nil, ErrInvalidInput
	}
	jobs := NewRiverJobs()
	var workers *river.Workers
	if execute {
		workers = river.NewWorkers()
	}
	service, err := NewService(NewPostgresStore(pool), jobs, options)
	if err != nil {
		return nil, nil, err
	}
	if execute {
		if err := RegisterRiverWorkers(workers, service); err != nil {
			return nil, nil, err
		}
	}
	client, err := NewRiverClient(pool, workers, execute)
	if err != nil {
		return nil, nil, err
	}
	if err := jobs.SetClient(client); err != nil {
		return nil, nil, err
	}
	return service, client, nil
}

type analysisWorker struct {
	river.WorkerDefaults[AnalysisArgs]
	service *Service
}

func (worker *analysisWorker) Work(ctx context.Context, job *river.Job[AnalysisArgs]) error {
	id, err := ParseIdentifier(job.Args.AnalysisID)
	if err != nil {
		return river.JobCancel(err)
	}
	err = worker.service.RunAnalysis(ctx, id)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCancelled) || errors.Is(err, ErrInvalidInput) || errors.Is(err, ErrInvalidState) ||
		errors.Is(err, ErrNotFound) || errors.Is(err, ErrTimeout) {
		return river.JobCancel(err)
	}
	return err
}
