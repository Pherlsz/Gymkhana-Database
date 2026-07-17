package operations

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

const OperationsQueue = "operations"

type ParseImportArgs struct {
	ImportID string `json:"import_id" river:"unique"`
}

func (ParseImportArgs) Kind() string { return "operations_parse_import" }

type ExecuteImportArgs struct {
	ImportID string `json:"import_id" river:"unique"`
}

func (ExecuteImportArgs) Kind() string { return "operations_execute_import" }

type GenerateExportArgs struct {
	ExportID string `json:"export_id" river:"unique"`
}

func (GenerateExportArgs) Kind() string { return "operations_generate_export" }

type CleanupArgs struct {
	Window string `json:"window" river:"unique"`
}

func (CleanupArgs) Kind() string { return "operations_cleanup" }

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

func (jobs *RiverJobs) EnqueueParse(ctx context.Context, id Identifier) (int64, error) {
	return jobs.insert(ctx, ParseImportArgs{ImportID: id.String()})
}

func (jobs *RiverJobs) EnqueueExecute(ctx context.Context, id Identifier) (int64, error) {
	return jobs.insert(ctx, ExecuteImportArgs{ImportID: id.String()})
}

func (jobs *RiverJobs) EnqueueExport(ctx context.Context, id Identifier) (int64, error) {
	return jobs.insert(ctx, GenerateExportArgs{ExportID: id.String()})
}

func (jobs *RiverJobs) EnqueueCleanup(ctx context.Context, window time.Time) (int64, error) {
	if window.IsZero() {
		return 0, ErrInvalidInput
	}
	return jobs.insert(ctx, CleanupArgs{Window: window.UTC().Format(time.RFC3339)})
}

func (jobs *RiverJobs) insert(ctx context.Context, args river.JobArgs) (int64, error) {
	client, err := jobs.readyClient()
	if err != nil {
		return 0, err
	}
	result, err := client.Insert(ctx, args, &river.InsertOpts{
		MaxAttempts: 5,
		Queue:       OperationsQueue,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
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
		return fmt.Errorf("cancel operation job: %w", err)
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
		JobTimeout:           20 * time.Minute,
		RescueStuckJobsAfter: 30 * time.Minute,
		SoftStopTimeout:      30 * time.Second,
	}
	if execute {
		if workers == nil {
			return nil, ErrInvalidInput
		}
		config.Workers = workers
		config.Queues = map[string]river.QueueConfig{
			OperationsQueue: {MaxWorkers: 4},
		}
	} else {
		config.SkipUnknownJobCheck = true
	}
	client, err := river.NewClient(riverpgxv5.New(pool), config)
	if err != nil {
		return nil, fmt.Errorf("configure operation queue: %w", err)
	}
	return client, nil
}

func RegisterRiverWorkers(workers *river.Workers, service *Service) error {
	if workers == nil || service == nil {
		return ErrInvalidInput
	}
	river.AddWorker(workers, &parseImportWorker{service: service})
	river.AddWorker(workers, &executeImportWorker{service: service})
	river.AddWorker(workers, &generateExportWorker{service: service})
	river.AddWorker(workers, &cleanupWorker{service: service})
	return nil
}

func NewRuntime(pool *pgxpool.Pool, objects ObjectStore, options ServiceOptions, execute bool) (*Service, *river.Client[pgx.Tx], error) {
	if pool == nil || objects == nil {
		return nil, nil, ErrInvalidInput
	}
	jobs := NewRiverJobs()
	var workers *river.Workers
	if execute {
		workers = river.NewWorkers()
	}
	service, err := NewService(NewPostgresStore(pool), objects, jobs, options)
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

type parseImportWorker struct {
	river.WorkerDefaults[ParseImportArgs]
	service *Service
}

func (worker *parseImportWorker) Work(ctx context.Context, job *river.Job[ParseImportArgs]) error {
	id, err := ParseIdentifier(job.Args.ImportID)
	if err != nil {
		return river.JobCancel(err)
	}
	return riverWorkerResult(worker.service.ParseImport(ctx, id))
}

type executeImportWorker struct {
	river.WorkerDefaults[ExecuteImportArgs]
	service *Service
}

func (worker *executeImportWorker) Work(ctx context.Context, job *river.Job[ExecuteImportArgs]) error {
	id, err := ParseIdentifier(job.Args.ImportID)
	if err != nil {
		return river.JobCancel(err)
	}
	return riverWorkerResult(worker.service.ExecuteImport(ctx, id))
}

type generateExportWorker struct {
	river.WorkerDefaults[GenerateExportArgs]
	service *Service
}

type cleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	service *Service
}

func (worker *cleanupWorker) Work(ctx context.Context, _ *river.Job[CleanupArgs]) error {
	_, err := worker.service.Cleanup(ctx)
	return riverWorkerResult(err)
}

func (worker *generateExportWorker) Work(ctx context.Context, job *river.Job[GenerateExportArgs]) error {
	id, err := ParseIdentifier(job.Args.ExportID)
	if err != nil {
		return river.JobCancel(err)
	}
	return riverWorkerResult(worker.service.GenerateExport(ctx, id))
}

func riverWorkerResult(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrCancelled) || errors.Is(err, ErrConflict) || errors.Is(err, ErrExpired) ||
		errors.Is(err, ErrForbidden) ||
		errors.Is(err, ErrStalePreview) || errors.Is(err, ErrUnsupportedWorkbook) || errors.Is(err, ErrWorkbookLimit) {
		return river.JobCancel(err)
	}
	return err
}
