package ocr

import (
	"context"
	"errors"
	"testing"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/riverqueue/river"
)

func TestExtractionWorkerRetriesOnlyPreProviderUnavailableAttempts(t *testing.T) {
	fixture := newOCRServiceFixture(t, NewDeterministicFakeExtractor())
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-worker-retry-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}
	fixture.source.failGet = map[int]error{2: attachment.ErrStorageUnavailable}

	err = (&extractionWorker{service: fixture.service}).Work(context.Background(), extractionRiverJob(job.ID))
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Work() error = %v, want retryable ErrUnavailable", err)
	}
	var cancelError *river.JobCancelError
	if errors.As(err, &cancelError) {
		t.Fatalf("pre-provider failure was cancelled instead of retried: %v", err)
	}
	if fixture.store.job.State != JobRunning || fixture.store.job.AttemptCount != 1 {
		t.Fatalf("retryable worker job = %#v", fixture.store.job)
	}
}

func TestExtractionWorkerCancelsAmbiguousProviderFailure(t *testing.T) {
	extractor := NewFakeExtractor(FakeExtractionStep{Err: errors.New("ambiguous provider transport")})
	fixture := newOCRServiceFixture(t, extractor)
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-worker-provider-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	err = (&extractionWorker{service: fixture.service}).Work(context.Background(), extractionRiverJob(job.ID))
	var cancelError *river.JobCancelError
	if !errors.As(err, &cancelError) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Work() error = %v, want JobCancelError wrapping ErrUnavailable", err)
	}
	if fixture.store.job.State != JobFailed || len(extractor.Requests()) != 1 {
		t.Fatalf("terminal worker job=%#v provider requests=%d", fixture.store.job, len(extractor.Requests()))
	}
}

func TestExtractionWorkerHonorsConcurrentCancellationWithoutRetry(t *testing.T) {
	fixture := newOCRServiceFixture(t, NewDeterministicFakeExtractor())
	fixture.service.extractor = extractorFunc(func(_ context.Context, _ ExtractionRequest) (ExtractionResponse, error) {
		now := fixture.service.now().UTC()
		fixture.store.job.CancelRequestedAt = &now
		return ExtractionResponse{Usage: 1}, nil
	})
	job, err := fixture.service.StartJob(context.Background(), fixture.actor, fixture.source.value.ID, "ocr-worker-cancel-0001", nil, "start")
	if err != nil {
		t.Fatalf("StartJob() error = %v", err)
	}

	err = (&extractionWorker{service: fixture.service}).Work(context.Background(), extractionRiverJob(job.ID))
	var cancelError *river.JobCancelError
	if !errors.As(err, &cancelError) || !errors.Is(err, ErrCancelled) {
		t.Fatalf("Work() error = %v, want JobCancelError wrapping ErrCancelled", err)
	}
	if fixture.store.job.State != JobCancelled || fixture.store.job.ProviderUsage != 1 {
		t.Fatalf("cancelled worker job = %#v", fixture.store.job)
	}
}

func TestExtractionWorkerAndQueueFailClosedOnInvalidConfiguration(t *testing.T) {
	worker := &extractionWorker{}
	err := worker.Work(context.Background(), &river.Job[ExtractionArgs]{Args: ExtractionArgs{JobID: "not-an-identifier"}})
	var cancelError *river.JobCancelError
	if !errors.As(err, &cancelError) {
		t.Fatalf("Work(invalid args) error = %v, want JobCancelError", err)
	}

	jobs := NewRiverJobs()
	id, _ := NewIdentifier()
	if _, err := jobs.EnqueueExtraction(context.Background(), id); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("EnqueueExtraction(unconfigured) error = %v, want ErrInvalidState", err)
	}
	if err := RegisterRiverWorkers(nil, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RegisterRiverWorkers(nil) error = %v, want ErrInvalidInput", err)
	}
}

func extractionRiverJob(id Identifier) *river.Job[ExtractionArgs] {
	return &river.Job[ExtractionArgs]{Args: ExtractionArgs{JobID: id.String()}}
}
