package httpserver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/matching"
)

type matchingService interface {
	Catalog(auth.Session) ([]matching.EvidenceDefinition, error)
	StartAnalysis(context.Context, auth.Session, string, string) (matching.Analysis, error)
	Analysis(context.Context, auth.Session, matching.Identifier) (matching.Analysis, error)
	CancelAnalysis(context.Context, auth.Session, matching.Identifier, string) (matching.Analysis, error)
	ListCases(context.Context, auth.Session, matching.CaseListOptions) (matching.CasePage, error)
	Case(context.Context, auth.Session, matching.Identifier, string) (matching.Case, error)
	DismissCase(context.Context, auth.Session, matching.Identifier, int64, string) (matching.Case, error)
	PreviewMerge(context.Context, auth.Session, matching.MergePreviewInput, string) (matching.MergePreview, error)
	Merge(context.Context, auth.Session, matching.MergeInput, string) (matching.MergeResult, error)
}

type matchingAnalysisRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
}

type matchingDismissRequest struct {
	Version int64 `json:"version"`
}

type matchingFieldChoiceRequest struct {
	FieldKey string               `json:"field_key"`
	Source   matching.FieldSource `json:"source"`
}

type matchingPreviewRequest struct {
	SurvivorProfileID string                       `json:"survivor_profile_id"`
	SourceProfileID   string                       `json:"source_profile_id"`
	SurvivorVersion   int64                        `json:"survivor_version"`
	SourceVersion     int64                        `json:"source_version"`
	Choices           []matchingFieldChoiceRequest `json:"choices"`
}

type matchingMergeRequest struct {
	SurvivorProfileID  string                       `json:"survivor_profile_id"`
	SourceProfileID    string                       `json:"source_profile_id"`
	SurvivorVersion    int64                        `json:"survivor_version"`
	SourceVersion      int64                        `json:"source_version"`
	Choices            []matchingFieldChoiceRequest `json:"choices"`
	PreviewFingerprint string                       `json:"preview_fingerprint"`
	IdempotencyKey     string                       `json:"idempotency_key"`
	Confirmation       string                       `json:"confirmation"`
}

type matchingCatalogResponse struct {
	Evidence                 []matching.EvidenceDefinition `json:"evidence"`
	CanMerge                 bool                          `json:"can_merge"`
	MaximumCandidates        int                           `json:"maximum_candidates"`
	MaximumPageSize          int                           `json:"maximum_page_size"`
	MaximumAnalysesPerWindow int                           `json:"maximum_analyses_per_window"`
	AnalysisWindowSeconds    int                           `json:"analysis_window_seconds"`
}

type matchingAnalysisResponse struct {
	ID                string                 `json:"id"`
	State             matching.AnalysisState `json:"state"`
	ProfilesScanned   int                    `json:"profiles_scanned"`
	CandidateCount    int                    `json:"candidate_count"`
	RefreshedCount    int                    `json:"refreshed_count"`
	ErrorCode         string                 `json:"error_code,omitempty"`
	CancelRequestedAt *time.Time             `json:"cancel_requested_at,omitempty"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	CompletedAt       *time.Time             `json:"completed_at,omitempty"`
	ExpiresAt         time.Time              `json:"expires_at"`
	Version           int64                  `json:"version"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

type matchingEvidenceResponse struct {
	Kind         matching.EvidenceKind `json:"kind"`
	Strength     int                   `json:"strength"`
	Contribution int                   `json:"contribution"`
}

type matchingProfileResponse struct {
	ID                  string    `json:"id"`
	FullName            string    `json:"full_name"`
	SocialName          string    `json:"social_name,omitempty"`
	CPF                 string    `json:"cpf,omitempty"`
	Email               string    `json:"email,omitempty"`
	MobilePhone         string    `json:"mobile_phone,omitempty"`
	LandlinePhone       string    `json:"landline_phone,omitempty"`
	AddressStreet       string    `json:"address_street,omitempty"`
	AddressNumber       string    `json:"address_number,omitempty"`
	AddressComplement   string    `json:"address_complement,omitempty"`
	AddressNeighborhood string    `json:"address_neighborhood,omitempty"`
	AddressCity         string    `json:"address_city,omitempty"`
	AddressState        string    `json:"address_state,omitempty"`
	AddressPostalCode   string    `json:"address_postal_code,omitempty"`
	Notes               string    `json:"notes,omitempty"`
	Version             int64     `json:"version"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type matchingCaseResponse struct {
	ID                  string                     `json:"id"`
	LeftProfileID       string                     `json:"left_profile_id"`
	RightProfileID      string                     `json:"right_profile_id"`
	LeftProfileVersion  int64                      `json:"left_profile_version"`
	RightProfileVersion int64                      `json:"right_profile_version"`
	Score               int                        `json:"score"`
	ScoreBand           matching.ScoreBand         `json:"score_band"`
	State               matching.CaseState         `json:"state"`
	Evidence            []matchingEvidenceResponse `json:"evidence"`
	Left                *matchingProfileResponse   `json:"left,omitempty"`
	Right               *matchingProfileResponse   `json:"right,omitempty"`
	MergedSurvivorID    string                     `json:"merged_survivor_id,omitempty"`
	MergedSourceID      string                     `json:"merged_source_id,omitempty"`
	MergedAt            *time.Time                 `json:"merged_at,omitempty"`
	Version             int64                      `json:"version"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
}

type matchingCasePageResponse struct {
	Cases  []matchingCaseResponse `json:"cases"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

type matchingMergeFieldResponse struct {
	Key            string               `json:"key"`
	Label          string               `json:"label"`
	Kind           string               `json:"kind"`
	SurvivorValue  *string              `json:"survivor_value"`
	SourceValue    *string              `json:"source_value"`
	Conflict       bool                 `json:"conflict"`
	ChoiceRequired bool                 `json:"choice_required"`
	SelectedSource matching.FieldSource `json:"selected_source,omitempty"`
}

type matchingMergePreviewResponse struct {
	CaseID               string                        `json:"case_id"`
	Survivor             matchingProfileResponse       `json:"survivor"`
	Source               matchingProfileResponse       `json:"source"`
	Fields               []matchingMergeFieldResponse  `json:"fields"`
	Dependencies         []matching.DependencyCount    `json:"dependencies"`
	Conflicts            []matching.DependencyConflict `json:"conflicts"`
	UnresolvedFieldCount int                           `json:"unresolved_field_count"`
	PreviewFingerprint   string                        `json:"preview_fingerprint"`
	Confirmation         string                        `json:"confirmation"`
	GeneratedAt          time.Time                     `json:"generated_at"`
}

type matchingMergeResultResponse struct {
	CaseID            string                     `json:"case_id"`
	SurvivorProfileID string                     `json:"survivor_profile_id"`
	SourceProfileID   string                     `json:"source_profile_id"`
	SurvivorVersion   int64                      `json:"survivor_version"`
	MovedDependencies []matching.DependencyCount `json:"moved_dependencies"`
	MergedAt          time.Time                  `json:"merged_at"`
}

func registerMatchingRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service matchingService) {
	mux.HandleFunc("GET /api/v1/matching/catalog", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := matchingActor(w, r, authentication, service)
		if !ok {
			return
		}
		catalog, err := service.Catalog(actor)
		if err != nil {
			writeMatchingError(w, r, logger, "read Matching catalog", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingCatalogResponse{
			Evidence: catalog, CanMerge: actor.User.Role.CanMergeProfiles(), MaximumCandidates: matching.MaximumCandidates,
			MaximumPageSize: matching.MaximumCasePageSize, MaximumAnalysesPerWindow: matching.MaximumAnalysisRate,
			AnalysisWindowSeconds: int(matching.AnalysisWindow.Seconds()),
		})
	})

	mux.HandleFunc("POST /api/v1/matching/analyses", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := matchingActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request matchingAnalysisRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.StartAnalysis(r.Context(), actor, request.IdempotencyKey, requestIDFromContext(r.Context()))
		if err != nil {
			writeMatchingError(w, r, logger, "start Matching analysis", err)
			return
		}
		writeJSON(w, http.StatusAccepted, matchingAnalysisFromDomain(value))
	})

	mux.HandleFunc("GET /api/v1/matching/analyses/{analysis_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := matchingActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := matchingIdentifier(r.PathValue("analysis_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Analysis(r.Context(), actor, id)
		if err != nil {
			writeMatchingError(w, r, logger, "read Matching analysis", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingAnalysisFromDomain(value))
	})

	mux.HandleFunc("POST /api/v1/matching/analyses/{analysis_id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := matchingActor(w, r, authentication, service)
		if !ok {
			return
		}
		id, problem := matchingIdentifier(r.PathValue("analysis_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.CancelAnalysis(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeMatchingError(w, r, logger, "cancel Matching analysis", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingAnalysisFromDomain(value))
	})

	registerMatchingCaseRoutes(mux, logger, authentication, service)
}

func registerMatchingCaseRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, service matchingService) {
	mux.HandleFunc("GET /api/v1/matching/cases", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := matchingActor(w, r, authentication, service)
		if !ok {
			return
		}
		options, problem := matchingCaseOptions(r)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		page, err := service.ListCases(r.Context(), actor, options)
		if err != nil {
			writeMatchingError(w, r, logger, "list Matching cases", err)
			return
		}
		response := matchingCasePageResponse{Cases: make([]matchingCaseResponse, 0, len(page.Cases)), Total: page.Total, Limit: page.Limit, Offset: page.Offset}
		for _, value := range page.Cases {
			response.Cases = append(response.Cases, matchingCaseFromDomain(value, false))
		}
		writeJSON(w, http.StatusOK, response)
	})

	mux.HandleFunc("GET /api/v1/matching/cases/{case_id}", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := matchingCaseActor(w, r, authentication, service)
		if !ok {
			return
		}
		value, err := service.Case(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeMatchingError(w, r, logger, "read Matching case", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingCaseFromDomain(value, true))
	})

	mux.HandleFunc("POST /api/v1/matching/cases/{case_id}/dismiss", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := matchingCaseActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request matchingDismissRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.DismissCase(r.Context(), actor, id, request.Version, requestIDFromContext(r.Context()))
		if err != nil {
			writeMatchingError(w, r, logger, "dismiss Matching case", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingCaseFromDomain(value, true))
	})

	mux.HandleFunc("POST /api/v1/matching/cases/{case_id}/merge-preview", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := matchingCaseActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request matchingPreviewRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		input, problem := matchingPreviewInput(id, request)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.PreviewMerge(r.Context(), actor, input, requestIDFromContext(r.Context()))
		if err != nil {
			writeMatchingError(w, r, logger, "preview Profile merge", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingPreviewFromDomain(value))
	})

	mux.HandleFunc("POST /api/v1/matching/cases/{case_id}/merge", func(w http.ResponseWriter, r *http.Request) {
		actor, id, ok := matchingCaseActor(w, r, authentication, service)
		if !ok {
			return
		}
		var request matchingMergeRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		preview, problem := matchingPreviewInput(id, matchingPreviewRequest{
			SurvivorProfileID: request.SurvivorProfileID, SourceProfileID: request.SourceProfileID,
			SurvivorVersion: request.SurvivorVersion, SourceVersion: request.SourceVersion, Choices: request.Choices,
		})
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		fingerprint, problem := matchingFingerprint(request.PreviewFingerprint)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Merge(r.Context(), actor, matching.MergeInput{
			MergePreviewInput: preview, PreviewFingerprint: fingerprint,
			IdempotencyKey: request.IdempotencyKey, Confirmation: request.Confirmation,
		}, requestIDFromContext(r.Context()))
		if err != nil {
			writeMatchingError(w, r, logger, "merge Profiles", err)
			return
		}
		writeJSON(w, http.StatusOK, matchingMergeResultResponse{
			CaseID: value.CaseID.String(), SurvivorProfileID: value.SurvivorProfileID.String(), SourceProfileID: value.SourceProfileID.String(),
			SurvivorVersion: value.SurvivorVersion, MovedDependencies: value.MovedDependencies, MergedAt: value.MergedAt,
		})
	})
}

func matchingActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service matchingService) (auth.Session, bool) {
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "A revisão de duplicatas não está configurada"})
		return auth.Session{}, false
	}
	return actor, true
}

func matchingCaseActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service matchingService) (auth.Session, matching.Identifier, bool) {
	actor, ok := matchingActor(w, r, authentication, service)
	if !ok {
		return auth.Session{}, matching.Identifier{}, false
	}
	id, problem := matchingIdentifier(r.PathValue("case_id"))
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, matching.Identifier{}, false
	}
	return actor, id, true
}

func matchingIdentifier(value string) (matching.Identifier, *Problem) {
	id, err := matching.ParseIdentifier(value)
	if err != nil {
		return matching.Identifier{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Identificador de matching inválido"}
	}
	return id, nil
}

func matchingFingerprint(value string) ([sha256.Size]byte, *Problem) {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) != sha256.Size {
		return [sha256.Size]byte{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Fingerprint do preview inválido"}
	}
	var result [sha256.Size]byte
	copy(result[:], decoded)
	return result, nil
}

func matchingPreviewInput(caseID matching.Identifier, request matchingPreviewRequest) (matching.MergePreviewInput, *Problem) {
	survivor, problem := matchingIdentifier(request.SurvivorProfileID)
	if problem != nil {
		return matching.MergePreviewInput{}, problem
	}
	source, problem := matchingIdentifier(request.SourceProfileID)
	if problem != nil {
		return matching.MergePreviewInput{}, problem
	}
	choices := make([]matching.FieldChoice, 0, len(request.Choices))
	for _, choice := range request.Choices {
		choices = append(choices, matching.FieldChoice{FieldKey: choice.FieldKey, Source: choice.Source})
	}
	return matching.MergePreviewInput{
		CaseID: caseID, SurvivorID: survivor, SourceID: source,
		SurvivorVersion: request.SurvivorVersion, SourceVersion: request.SourceVersion, Choices: choices,
	}, nil
}

func matchingCaseOptions(r *http.Request) (matching.CaseListOptions, *Problem) {
	limit, offset := matching.MaximumCasePageSize, 0
	for key, destination := range map[string]*int{"limit": &limit, "offset": &offset} {
		if raw := strings.TrimSpace(r.URL.Query().Get(key)); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				return matching.CaseListOptions{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Paginação de matching inválida"}
			}
			*destination = value
		}
	}
	states := make([]matching.CaseState, 0)
	for _, raw := range splitMatchingFilter(r.URL.Query().Get("state")) {
		states = append(states, matching.CaseState(raw))
	}
	bands := make([]matching.ScoreBand, 0)
	for _, raw := range splitMatchingFilter(r.URL.Query().Get("score_band")) {
		bands = append(bands, matching.ScoreBand(raw))
	}
	return matching.CaseListOptions{States: states, Bands: bands, Limit: limit, Offset: offset}, nil
}

func splitMatchingFilter(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalized := strings.ToUpper(strings.TrimSpace(part)); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func matchingAnalysisFromDomain(value matching.Analysis) matchingAnalysisResponse {
	return matchingAnalysisResponse{
		ID: value.ID.String(), State: value.State, ProfilesScanned: value.ProfilesScanned,
		CandidateCount: value.CandidateCount, RefreshedCount: value.RefreshedCount, ErrorCode: value.ErrorCode,
		CancelRequestedAt: value.CancelRequestedAt, StartedAt: value.StartedAt, CompletedAt: value.CompletedAt,
		ExpiresAt: value.ExpiresAt, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func matchingCaseFromDomain(value matching.Case, detailed bool) matchingCaseResponse {
	response := matchingCaseResponse{
		ID: value.ID.String(), LeftProfileID: value.LeftProfileID.String(), RightProfileID: value.RightProfileID.String(),
		LeftProfileVersion: value.LeftProfileVersion, RightProfileVersion: value.RightProfileVersion,
		Score: value.Score, ScoreBand: value.ScoreBand, State: value.State,
		Evidence: make([]matchingEvidenceResponse, 0, len(value.Evidence)), Version: value.Version,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, MergedAt: value.MergedAt,
	}
	for _, evidence := range value.Evidence {
		response.Evidence = append(response.Evidence, matchingEvidenceResponse{
			Kind: evidence.Kind, Strength: evidence.Strength, Contribution: evidence.Contribution,
		})
	}
	if value.Left != nil {
		converted := matchingProfileFromDomain(*value.Left, detailed)
		response.Left = &converted
	}
	if value.Right != nil {
		converted := matchingProfileFromDomain(*value.Right, detailed)
		response.Right = &converted
	}
	if value.MergedSurvivorID != nil {
		response.MergedSurvivorID = value.MergedSurvivorID.String()
	}
	if value.MergedSourceID != nil {
		response.MergedSourceID = value.MergedSourceID.String()
	}
	return response
}

func matchingProfileFromDomain(value matching.ProfileSnapshot, detailed bool) matchingProfileResponse {
	response := matchingProfileResponse{
		ID: value.ID.String(), FullName: value.FullName, SocialName: value.SocialName, CPF: value.CPF,
		Email: value.Email, MobilePhone: value.MobilePhone, LandlinePhone: value.LandlinePhone,
		AddressCity: value.AddressCity, AddressState: value.AddressState, Version: value.Version, UpdatedAt: value.UpdatedAt,
	}
	if detailed {
		response.AddressStreet, response.AddressNumber = value.AddressStreet, value.AddressNumber
		response.AddressComplement, response.AddressNeighborhood = value.AddressComplement, value.AddressNeighborhood
		response.AddressPostalCode, response.Notes = value.AddressPostalCode, value.Notes
	}
	return response
}

func matchingPreviewFromDomain(value matching.MergePreview) matchingMergePreviewResponse {
	response := matchingMergePreviewResponse{
		CaseID: value.CaseID.String(), Survivor: matchingProfileFromDomain(value.Survivor, true), Source: matchingProfileFromDomain(value.Source, true),
		Fields: make([]matchingMergeFieldResponse, 0, len(value.Fields)), Dependencies: value.Dependencies,
		Conflicts: value.Conflicts, UnresolvedFieldCount: value.UnresolvedFieldCount,
		PreviewFingerprint: hex.EncodeToString(value.PreviewFingerprint[:]), Confirmation: value.Confirmation, GeneratedAt: value.GeneratedAt,
	}
	for _, field := range value.Fields {
		response.Fields = append(response.Fields, matchingMergeFieldResponse{
			Key: field.Key, Label: field.Label, Kind: field.Kind, SurvivorValue: field.SurvivorValue,
			SourceValue: field.SourceValue, Conflict: field.Conflict, ChoiceRequired: field.ChoiceRequired, SelectedSource: field.SelectedSource,
		})
	}
	return response
}

func writeMatchingError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	switch {
	case errors.Is(err, matching.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para revisar ou mesclar estas pessoas"})
	case errors.Is(err, matching.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "Análise ou caso de duplicidade não encontrado"})
	case errors.Is(err, matching.ErrRateLimited):
		writeProblem(w, r, Problem{Status: http.StatusTooManyRequests, Code: ErrorCodeRateLimited, Message: "Limite de análises atingido. Aguarde antes de tentar novamente"})
	case errors.Is(err, matching.ErrTimeout):
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeMatchingTimeout, Message: "A análise excedeu o tempo seguro"})
	case errors.Is(err, matching.ErrCancelled):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeMatchingCancelled, Message: "A análise foi cancelada"})
	case errors.Is(err, matching.ErrStalePreview):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeMatchingStale, Message: "Os dados mudaram. Gere um novo preview antes de mesclar"})
	case errors.Is(err, matching.ErrDependencyConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeMatchingConflict, Message: "Há dependências incompatíveis que precisam ser resolvidas antes do merge"})
	case errors.Is(err, matching.ErrConflict), errors.Is(err, matching.ErrInvalidState):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "O caso mudou ou não aceita esta operação"})
	case errors.Is(err, matching.ErrInvalidInput), errors.Is(err, matching.ErrInvalidConfirmation):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Revise os identificadores, versões, escolhas e confirmação"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação de matching"})
	}
}
