package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/attachment"
	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/featureflags"
	ocrdomain "github.com/Pherlsz/Gymkhana-Database/internal/ocr"
)

const ocrStreamPollInterval = 250 * time.Millisecond
const ocrStreamHeartbeatInterval = 10 * time.Second

type ocrService interface {
	Capability() ocrdomain.Capability
	StartJob(context.Context, auth.Session, attachment.Identifier, string, *ocrdomain.Identifier, string) (ocrdomain.Job, error)
	Jobs(context.Context, auth.Session, int, int, string) (ocrdomain.JobPage, error)
	Job(context.Context, auth.Session, ocrdomain.Identifier, string) (ocrdomain.Job, error)
	CancelJob(context.Context, auth.Session, ocrdomain.Identifier, string) (ocrdomain.Job, error)
	Events(context.Context, auth.Session, ocrdomain.Identifier, int64, int) (ocrdomain.EventPage, error)
	Suggestions(context.Context, auth.Session, ocrdomain.Identifier, int, int, string) (ocrdomain.SuggestionPage, error)
	ReviewSuggestion(context.Context, auth.Session, ocrdomain.Identifier, ocrdomain.ReviewInput, string) (ocrdomain.Suggestion, error)
	Apply(context.Context, auth.Session, ocrdomain.Identifier, []ocrdomain.ApplySelection, string, string) (ocrdomain.ApplyReceipt, error)
}

type ocrCapabilityResponse struct {
	Enabled                bool     `json:"enabled"`
	SupportedMIMEs         []string `json:"supported_mimes"`
	MaximumSourceBytes     int64    `json:"maximum_source_bytes"`
	MaximumPages           int      `json:"maximum_pages"`
	MaximumPixels          int64    `json:"maximum_pixels"`
	MaximumSuggestions     int      `json:"maximum_suggestions"`
	MaximumDurationSeconds int64    `json:"maximum_duration_seconds"`
	MaximumRequests        int      `json:"maximum_requests_per_hour"`
	MaximumProviderUsage   int64    `json:"maximum_provider_usage_per_hour"`
}

type ocrJobResponse struct {
	ID                string             `json:"id"`
	AttachmentID      string             `json:"attachment_id"`
	RetryOfJobID      *string            `json:"retry_of_job_id,omitempty"`
	SourceMIME        string             `json:"source_mime"`
	SourceBytes       int64              `json:"source_bytes"`
	State             ocrdomain.JobState `json:"state"`
	PageCount         int                `json:"page_count"`
	PixelCount        int64              `json:"pixel_count"`
	SuggestionCount   int                `json:"suggestion_count"`
	ProviderUsage     int64              `json:"provider_usage"`
	AttemptCount      int                `json:"attempt_count"`
	ErrorCode         string             `json:"error_code,omitempty"`
	CancelRequestedAt *time.Time         `json:"cancel_requested_at,omitempty"`
	StartedAt         *time.Time         `json:"started_at,omitempty"`
	CompletedAt       *time.Time         `json:"completed_at,omitempty"`
	Version           int64              `json:"version"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
}

type ocrJobPageResponse struct {
	Jobs   []ocrJobResponse `json:"jobs"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type ocrRegionResponse struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ocrEvidenceResponse struct {
	Page       int                `json:"page"`
	Region     *ocrRegionResponse `json:"region,omitempty"`
	Excerpt    string             `json:"excerpt,omitempty"`
	Confidence *int               `json:"confidence,omitempty"`
}

type ocrSuggestionResponse struct {
	ID            string                `json:"id"`
	JobID         string                `json:"job_id"`
	Ordinal       int                   `json:"ordinal"`
	TargetKind    ocrdomain.TargetKind  `json:"target_kind"`
	TargetID      string                `json:"target_id"`
	TargetVersion int64                 `json:"target_version"`
	FieldKey      string                `json:"field_key"`
	FieldLabel    string                `json:"field_label"`
	ValueKind     ocrdomain.ValueKind   `json:"value_kind"`
	ProposedValue string                `json:"proposed_value"`
	Evidence      ocrEvidenceResponse   `json:"evidence"`
	ReviewState   ocrdomain.ReviewState `json:"review_state"`
	ReviewedValue *string               `json:"reviewed_value,omitempty"`
	ReviewedAt    *time.Time            `json:"reviewed_at,omitempty"`
	AppliedAt     *time.Time            `json:"applied_at,omitempty"`
	Version       int64                 `json:"version"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
}

type ocrSuggestionViewResponse struct {
	Suggestion     ocrSuggestionResponse `json:"suggestion"`
	CurrentValue   string                `json:"current_value"`
	CurrentVersion int64                 `json:"current_version"`
	Stale          bool                  `json:"stale"`
}

type ocrSuggestionPageResponse struct {
	Suggestions []ocrSuggestionViewResponse `json:"suggestions"`
	Total       int                         `json:"total"`
	Limit       int                         `json:"limit"`
	Offset      int                         `json:"offset"`
}

type ocrEventResponse struct {
	Sequence        int64               `json:"sequence"`
	Kind            ocrdomain.EventKind `json:"kind"`
	PageCount       *int                `json:"page_count,omitempty"`
	SuggestionCount *int                `json:"suggestion_count,omitempty"`
	ErrorCode       string              `json:"error_code,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
}

type ocrApplyResultResponse struct {
	SuggestionID  string                 `json:"suggestion_id"`
	Outcome       ocrdomain.ApplyOutcome `json:"outcome"`
	TargetVersion *int64                 `json:"target_version,omitempty"`
	ErrorCode     string                 `json:"error_code,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

type ocrApplyReceiptResponse struct {
	ID          string                      `json:"id"`
	JobID       string                      `json:"job_id"`
	State       ocrdomain.ApplyReceiptState `json:"state"`
	Results     []ocrApplyResultResponse    `json:"results"`
	CreatedAt   time.Time                   `json:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at"`
	CompletedAt *time.Time                  `json:"completed_at,omitempty"`
}

type ocrStartJobRequest struct {
	AttachmentID   string  `json:"attachment_id"`
	IdempotencyKey string  `json:"idempotency_key"`
	RetryOfJobID   *string `json:"retry_of_job_id,omitempty"`
}

type ocrReviewRequest struct {
	Action  ocrdomain.ReviewAction `json:"action"`
	Value   *string                `json:"value,omitempty"`
	Version int64                  `json:"version"`
}

type ocrApplySelectionRequest struct {
	SuggestionID string `json:"suggestion_id"`
	Version      int64  `json:"version"`
}

type ocrApplyRequest struct {
	IdempotencyKey string                     `json:"idempotency_key"`
	Selections     []ocrApplySelectionRequest `json:"selections"`
}

func registerOCRRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service ocrService, flags featureFlagReader) {
	mux.HandleFunc("GET /api/v1/ocr/capability", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		if _, problem := authenticatedSession(r, authentication); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil || !featureFlagOn(r.Context(), flags, featureflags.KeyOCR) {
			writeJSON(w, http.StatusOK, ocrCapabilityFromDomain(ocrdomain.DefaultCapability()))
			return
		}
		writeJSON(w, http.StatusOK, ocrCapabilityFromDomain(service.Capability()))
	}))

	mux.HandleFunc("GET /api/v1/ocr/jobs", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		limit, offset, problem := ocrPagination(r, ocrdomain.MaximumJobPage)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Jobs(r.Context(), actor, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "list OCR jobs", err)
			return
		}
		writeJSON(w, http.StatusOK, ocrJobPageFromDomain(page))
	}))

	mux.HandleFunc("POST /api/v1/ocr/jobs", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		var request ocrStartJobRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		attachmentID, err := attachment.ParseIdentifier(request.AttachmentID)
		if err != nil {
			writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador do anexo inválido"})
			return
		}
		var retryID *ocrdomain.Identifier
		if request.RetryOfJobID != nil {
			parsed, err := ocrdomain.ParseIdentifier(*request.RetryOfJobID)
			if err != nil {
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador do job anterior inválido"})
				return
			}
			retryID = &parsed
		}
		job, err := service.StartJob(r.Context(), actor, attachmentID, request.IdempotencyKey, retryID, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "start OCR job", err)
			return
		}
		writeJSON(w, http.StatusAccepted, ocrJobFromDomain(job))
	}))

	mux.HandleFunc("GET /api/v1/ocr/jobs/{job_id}", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		id, ok := ocrPathIdentifier(w, r, "job_id")
		if !ok {
			return
		}
		job, err := service.Job(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "read OCR job", err)
			return
		}
		writeJSON(w, http.StatusOK, ocrJobFromDomain(job))
	}))

	mux.HandleFunc("POST /api/v1/ocr/jobs/{job_id}/cancel", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		id, ok := ocrPathIdentifier(w, r, "job_id")
		if !ok {
			return
		}
		job, err := service.CancelJob(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "cancel OCR job", err)
			return
		}
		writeJSON(w, http.StatusOK, ocrJobFromDomain(job))
	}))

	mux.HandleFunc("GET /api/v1/ocr/jobs/{job_id}/events", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		id, ok := ocrPathIdentifier(w, r, "job_id")
		if !ok {
			return
		}
		after, problem := ocrEventCursor(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		streamOCREvents(w, r, logger, service, actor, id, after)
	}))

	mux.HandleFunc("GET /api/v1/ocr/jobs/{job_id}/suggestions", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		id, ok := ocrPathIdentifier(w, r, "job_id")
		if !ok {
			return
		}
		limit, offset, problem := ocrPagination(r, ocrdomain.MaximumSuggestions)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.Suggestions(r.Context(), actor, id, limit, offset, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "list OCR suggestions", err)
			return
		}
		writeJSON(w, http.StatusOK, ocrSuggestionPageFromDomain(page))
	}))

	mux.HandleFunc("PATCH /api/v1/ocr/suggestions/{suggestion_id}", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		id, ok := ocrPathIdentifier(w, r, "suggestion_id")
		if !ok {
			return
		}
		var request ocrReviewRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.ReviewSuggestion(r.Context(), actor, id, ocrdomain.ReviewInput{
			Action: request.Action, Value: request.Value, Version: request.Version,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "review OCR suggestion", err)
			return
		}
		writeJSON(w, http.StatusOK, ocrSuggestionFromDomain(value))
	}))

	mux.HandleFunc("POST /api/v1/ocr/jobs/{job_id}/apply", requireCapability(auth.CapOCR, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, ok := configuredOCRActor(w, r, authentication, service, flags)
		if !ok {
			return
		}
		id, ok := ocrPathIdentifier(w, r, "job_id")
		if !ok {
			return
		}
		var request ocrApplyRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		selections := make([]ocrdomain.ApplySelection, 0, len(request.Selections))
		for _, selection := range request.Selections {
			suggestionID, err := ocrdomain.ParseIdentifier(selection.SuggestionID)
			if err != nil {
				writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Seleção de sugestão inválida"})
				return
			}
			selections = append(selections, ocrdomain.ApplySelection{SuggestionID: suggestionID, Version: selection.Version})
		}
		receipt, err := service.Apply(r.Context(), actor, id, selections, request.IdempotencyKey, requestIDFromContext(r.Context()))
		if err != nil {
			writeOCRError(w, r, logger, "apply OCR suggestions", err)
			return
		}
		writeJSON(w, http.StatusOK, ocrApplyReceiptFromDomain(receipt))
	}))
}

func configuredOCRActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service ocrService, flags featureFlagReader) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil || !featureFlagOn(r.Context(), flags, featureflags.KeyOCR) {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeOCRUnavailable,
			Message: "OCR está desativado até que um provedor e modelo sejam configurados"})
		return auth.Session{}, false
	}
	return actor, true
}

func streamOCREvents(w http.ResponseWriter, r *http.Request, logger *slog.Logger, service ocrService, actor auth.Session, jobID ocrdomain.Identifier, after int64) {
	page, err := service.Events(r.Context(), actor, jobID, after, ocrdomain.MaximumEventPage)
	if err != nil {
		writeOCRError(w, r, logger, "open OCR event stream", err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeOCRUnavailable, Message: "Streaming não está disponível neste servidor"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	controller := http.NewResponseController(w)
	refreshDeadline := func() { _ = controller.SetWriteDeadline(time.Now().Add(ocrStreamHeartbeatInterval + 5*time.Second)) }
	refreshDeadline()
	heartbeat := time.NewTicker(ocrStreamHeartbeatInterval)
	poll := time.NewTicker(ocrStreamPollInterval)
	defer heartbeat.Stop()
	defer poll.Stop()
	for {
		for _, event := range page.Events {
			refreshDeadline()
			encoded, marshalErr := json.Marshal(ocrEventFromDomain(event))
			if marshalErr != nil {
				logger.Error("encode OCR event", "request_id", requestIDFromContext(r.Context()), "error_code", "internal_error")
				return
			}
			if _, writeErr := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Kind, encoded); writeErr != nil {
				return
			}
			after = event.Sequence
		}
		flusher.Flush()
		if page.Terminal {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			refreshDeadline()
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-poll.C:
			page, err = service.Events(r.Context(), actor, jobID, after, ocrdomain.MaximumEventPage)
			if err != nil {
				logger.Warn("close OCR event stream", "request_id", requestIDFromContext(r.Context()), "error_code", string(ocrErrorCode(err)))
				return
			}
		}
	}
}

func ocrPathIdentifier(w http.ResponseWriter, r *http.Request, name string) (ocrdomain.Identifier, bool) {
	id, err := ocrdomain.ParseIdentifier(r.PathValue(name))
	if err != nil {
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador OCR inválido"})
		return ocrdomain.Identifier{}, false
	}
	return id, true
}

func ocrPagination(r *http.Request, maximum int) (int, int, *Problem) {
	limit, offset := maximum, 0
	for key, destination := range map[string]*int{"limit": &limit, "offset": &offset} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação OCR inválida"}
		}
		*destination = value
	}
	if limit < 1 || limit > maximum || offset < 0 || offset > 10_000 {
		return 0, 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação OCR fora dos limites"}
	}
	return limit, offset, nil
}

func ocrEventCursor(r *http.Request) (int64, *Problem) {
	var cursor int64
	for _, raw := range []string{r.Header.Get("Last-Event-ID"), r.URL.Query().Get("after")} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			return 0, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Cursor de eventos OCR inválido"}
		}
		if value > cursor {
			cursor = value
		}
	}
	return cursor, nil
}

func ocrCapabilityFromDomain(value ocrdomain.Capability) ocrCapabilityResponse {
	return ocrCapabilityResponse{
		Enabled: value.Enabled, SupportedMIMEs: value.SupportedMIMEs, MaximumSourceBytes: value.MaximumSourceBytes,
		MaximumPages: value.MaximumPages, MaximumPixels: value.MaximumPixels, MaximumSuggestions: value.MaximumSuggestions,
		MaximumDurationSeconds: int64(value.MaximumDuration / time.Second), MaximumRequests: value.MaximumRequests,
		MaximumProviderUsage: value.MaximumProviderUsage,
	}
}

func ocrJobFromDomain(value ocrdomain.Job) ocrJobResponse {
	return ocrJobResponse{
		ID: value.ID.String(), AttachmentID: value.AttachmentID.String(), RetryOfJobID: ocrIdentifierString(value.RetryOfJobID),
		SourceMIME: value.SourceMIME, SourceBytes: value.SourceBytes, State: value.State, PageCount: value.PageCount,
		PixelCount: value.PixelCount, SuggestionCount: value.SuggestionCount, ProviderUsage: value.ProviderUsage,
		AttemptCount: value.AttemptCount,
		ErrorCode:    value.ErrorCode, CancelRequestedAt: value.CancelRequestedAt, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ocrJobPageFromDomain(value ocrdomain.JobPage) ocrJobPageResponse {
	response := ocrJobPageResponse{Jobs: make([]ocrJobResponse, 0, len(value.Jobs)), Total: value.Total, Limit: value.Limit, Offset: value.Offset}
	for _, job := range value.Jobs {
		response.Jobs = append(response.Jobs, ocrJobFromDomain(job))
	}
	return response
}

func ocrSuggestionFromDomain(value ocrdomain.Suggestion) ocrSuggestionResponse {
	evidence := ocrEvidenceResponse{Page: value.Evidence.Page, Excerpt: value.Evidence.Excerpt, Confidence: value.Evidence.Confidence}
	if value.Evidence.Region != nil {
		evidence.Region = &ocrRegionResponse{X: value.Evidence.Region.X, Y: value.Evidence.Region.Y, Width: value.Evidence.Region.Width, Height: value.Evidence.Region.Height}
	}
	return ocrSuggestionResponse{
		ID: value.ID.String(), JobID: value.JobID.String(), Ordinal: value.Ordinal, TargetKind: value.Target.Kind,
		TargetID: value.Target.ID.String(), TargetVersion: value.TargetVersion, FieldKey: value.FieldKey,
		FieldLabel: value.FieldLabel, ValueKind: value.Kind, ProposedValue: value.ProposedValue, Evidence: evidence,
		ReviewState: value.ReviewState, ReviewedValue: value.ReviewedValue, ReviewedAt: value.ReviewedAt,
		AppliedAt: value.AppliedAt, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func ocrSuggestionPageFromDomain(value ocrdomain.SuggestionPage) ocrSuggestionPageResponse {
	response := ocrSuggestionPageResponse{Suggestions: make([]ocrSuggestionViewResponse, 0, len(value.Suggestions)), Total: value.Total, Limit: value.Limit, Offset: value.Offset}
	for _, view := range value.Suggestions {
		response.Suggestions = append(response.Suggestions, ocrSuggestionViewResponse{
			Suggestion: ocrSuggestionFromDomain(view.Suggestion), CurrentValue: view.CurrentValue,
			CurrentVersion: view.CurrentVersion, Stale: view.Stale,
		})
	}
	return response
}

func ocrEventFromDomain(value ocrdomain.JobEvent) ocrEventResponse {
	return ocrEventResponse{Sequence: value.Sequence, Kind: value.Kind, PageCount: value.PageCount,
		SuggestionCount: value.SuggestionCount, ErrorCode: value.ErrorCode, CreatedAt: value.CreatedAt}
}

func ocrApplyReceiptFromDomain(value ocrdomain.ApplyReceipt) ocrApplyReceiptResponse {
	response := ocrApplyReceiptResponse{ID: value.ID.String(), JobID: value.JobID.String(), State: value.State,
		Results: make([]ocrApplyResultResponse, 0, len(value.Results)), CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt, CompletedAt: value.CompletedAt}
	for _, result := range value.Results {
		response.Results = append(response.Results, ocrApplyResultResponse{SuggestionID: result.SuggestionID.String(),
			Outcome: result.Outcome, TargetVersion: result.TargetVersion, ErrorCode: result.ErrorCode, CreatedAt: result.CreatedAt})
	}
	return response
}

func ocrIdentifierString(value *ocrdomain.Identifier) *string {
	if value == nil {
		return nil
	}
	encoded := value.String()
	return &encoded
}

func writeOCRError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	code := ocrErrorCode(err)
	switch {
	case errors.Is(err, ocrdomain.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para processar este anexo"})
	case errors.Is(err, ocrdomain.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Job, sugestão ou anexo não encontrado"})
	case errors.Is(err, ocrdomain.ErrConflict), errors.Is(err, ocrdomain.ErrInvalidState):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O recurso OCR mudou simultaneamente ou não está neste estado"})
	case errors.Is(err, ocrdomain.ErrStaleTarget):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeOCRStaleTarget, Message: "O cadastro de destino mudou; revise a sugestão novamente"})
	case errors.Is(err, ocrdomain.ErrInvalidInput):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Revise os dados enviados ao OCR"})
	case errors.Is(err, ocrdomain.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de extrações atingido. Aguarde antes de tentar novamente"})
	case errors.Is(err, ocrdomain.ErrQuotaExceeded):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeOCRQuota, Message: "A cota segura do provedor OCR foi atingida"})
	case errors.Is(err, ocrdomain.ErrUnsafeSource):
		writeProblem(w, r, Problem{Status: http.StatusUnsupportedMediaType, Code: ErrorCodeOCRUnsafeSource, Message: "O anexo não atende aos limites ou formatos seguros do OCR"})
	case errors.Is(err, ocrdomain.ErrCancelled):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeOCRCancelled, Message: "O job OCR foi cancelado"})
	case errors.Is(err, ocrdomain.ErrTimeout):
		writeProblem(w, r, Problem{Status: http.StatusGatewayTimeout, Code: ErrorCodeOCRTimeout, Message: "A extração excedeu o tempo seguro"})
	case errors.Is(err, ocrdomain.ErrUnavailable):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeOCRUnavailable, Message: "O processamento OCR está indisponível"})
	case errors.Is(err, ocrdomain.ErrMalformedProvider):
		writeProblem(w, r, Problem{Status: http.StatusBadGateway, Code: ErrorCodeOCRMalformed, Message: "O provedor OCR retornou uma resposta fora do contrato seguro"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error_code", string(code))
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação OCR"})
	}
}

func ocrErrorCode(err error) ErrorCode {
	switch {
	case errors.Is(err, ocrdomain.ErrQuotaExceeded):
		return ErrorCodeOCRQuota
	case errors.Is(err, ocrdomain.ErrStaleTarget):
		return ErrorCodeOCRStaleTarget
	case errors.Is(err, ocrdomain.ErrCancelled):
		return ErrorCodeOCRCancelled
	case errors.Is(err, ocrdomain.ErrTimeout):
		return ErrorCodeOCRTimeout
	case errors.Is(err, ocrdomain.ErrUnavailable):
		return ErrorCodeOCRUnavailable
	case errors.Is(err, ocrdomain.ErrMalformedProvider):
		return ErrorCodeOCRMalformed
	case errors.Is(err, ocrdomain.ErrUnsafeSource):
		return ErrorCodeOCRUnsafeSource
	default:
		return ErrorCodeInternal
	}
}
