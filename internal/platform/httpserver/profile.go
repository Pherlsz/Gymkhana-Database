package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	searchdomain "github.com/Pherlsz/Gymkhana-Database/internal/search"
)

type profileService interface {
	List(context.Context, auth.Session, profile.ListOptions) (profile.Page, error)
	Get(context.Context, auth.Session, profile.Identifier) (profile.Profile, error)
	DistinctCities(context.Context, auth.Session, profile.Filters, int32) ([]string, error)
	Create(context.Context, auth.Session, profile.Values, string) (profile.Profile, error)
	Update(context.Context, auth.Session, profile.Identifier, int64, profile.Values, string) (profile.Profile, error)
	Duplicate(context.Context, auth.Session, profile.Identifier, string) (profile.Profile, error)
	Delete(context.Context, auth.Session, profile.Identifier, int64, string, string) error
}

type profileAddressRequest struct {
	Street       string `json:"street"`
	Number       string `json:"number"`
	Complement   string `json:"complement"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postal_code"`
}

type profileValuesRequest struct {
	FullName        string                `json:"full_name"`
	SocialName      string                `json:"social_name"`
	CPF             string                `json:"cpf"`
	Email           string                `json:"email"`
	MobilePhone     string                `json:"mobile_phone"`
	LandlinePhone   string                `json:"landline_phone"`
	Address         profileAddressRequest `json:"address"`
	Notes           string                `json:"notes"`
	BirthDate       string                `json:"birth_date"`
	Gender          string                `json:"gender"`
	BloodType       string                `json:"blood_type"`
	Nationality     string                `json:"nationality"`
	BirthCity       string                `json:"birth_city"`
	MaritalStatus   string                `json:"marital_status"`
	WeddingDate     string                `json:"wedding_date"`
	FatherName      string                `json:"father_name"`
	FatherBirthDate string                `json:"father_birth_date"`
	MotherName      string                `json:"mother_name"`
	MotherBirthDate string                `json:"mother_birth_date"`
	HealthPlan      string                `json:"health_plan"`
	BloodDonor      *bool                 `json:"blood_donor"`
	OrganDonor      *bool                 `json:"organ_donor"`
	Team            string                `json:"team"`
	Sector          string                `json:"sector"`
	Collections     string                `json:"collections"`
	VehicleModel    string                `json:"vehicle_model"`
	VehicleColor    string                `json:"vehicle_color"`
	VehiclePlate    string                `json:"vehicle_plate"`
	VehicleYear     *int32                `json:"vehicle_year"`
	ClubMembership  string                `json:"club_membership"`
	MembershipType  string                `json:"membership_type"`
	PlaceOfOrigin   string                `json:"place_of_origin"`
	BirthCountry    string                `json:"birth_country"`
	ParentsWedding  string                `json:"parents_wedding_date"`
	SupermarketClub string                `json:"supermarket_club"`
	Pet             string                `json:"pet"`
	TravelCountries string                `json:"travel_countries"`
	CardBrand       string                `json:"card_brand"`
	CardBank        string                `json:"card_bank"`
}

type updateProfileRequest struct {
	profileValuesRequest
	Version int64 `json:"version"`
}

type deleteProfileRequest struct {
	Version      int64  `json:"version"`
	Confirmation string `json:"confirmation"`
}

type profileAddressResponse struct {
	Street       string `json:"street"`
	Number       string `json:"number"`
	Complement   string `json:"complement"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostalCode   string `json:"postal_code"`
}

type profileDocumentBadge struct {
	DocumentTypeID  string `json:"document_type_id"`
	TechnicalKey    string `json:"technical_key"`
	Label           string `json:"label"`
	Claim           string `json:"claim"`
	Badge           string `json:"badge"`
	IdentifierValue string `json:"identifier_value,omitempty"`
	HasPhysical     bool   `json:"has_physical"`
	HasDigital      bool   `json:"has_digital"`
	IdleCustody     string `json:"idle_custody,omitempty"`
	InHands         bool   `json:"in_hands"`
}

type profileDocumentPresence struct {
	DocumentTypeID  string `json:"document_type_id"`
	TechnicalKey    string `json:"technical_key"`
	Label           string `json:"label"`
	Claim           string `json:"claim"`
	IdentifierValue string `json:"identifier_value,omitempty"`
	HasPhysical     bool   `json:"has_physical"`
	HasDigital      bool   `json:"has_digital"`
}

type profileResponse struct {
	ID                  string                    `json:"id"`
	FullName            string                    `json:"full_name"`
	SocialName          string                    `json:"social_name"`
	CPF                 string                    `json:"cpf"`
	CPFDigitSum         *int                      `json:"cpf_digit_sum,omitempty"`
	Email               string                    `json:"email"`
	MobilePhone         string                    `json:"mobile_phone"`
	LandlinePhone       string                    `json:"landline_phone"`
	Address             profileAddressResponse    `json:"address"`
	Notes               string                    `json:"notes"`
	BirthDate           string                    `json:"birth_date,omitempty"`
	Gender              string                    `json:"gender,omitempty"`
	BloodType           string                    `json:"blood_type,omitempty"`
	Nationality         string                    `json:"nationality,omitempty"`
	BirthCity           string                    `json:"birth_city,omitempty"`
	MaritalStatus       string                    `json:"marital_status,omitempty"`
	WeddingDate         string                    `json:"wedding_date,omitempty"`
	FatherName          string                    `json:"father_name,omitempty"`
	FatherBirthDate     string                    `json:"father_birth_date,omitempty"`
	MotherName          string                    `json:"mother_name,omitempty"`
	MotherBirthDate     string                    `json:"mother_birth_date,omitempty"`
	HealthPlan          string                    `json:"health_plan,omitempty"`
	BloodDonor          *bool                     `json:"blood_donor,omitempty"`
	OrganDonor          *bool                     `json:"organ_donor,omitempty"`
	Team                string                    `json:"team,omitempty"`
	Sector              string                    `json:"sector,omitempty"`
	Collections         string                    `json:"collections,omitempty"`
	VehicleModel        string                    `json:"vehicle_model,omitempty"`
	VehicleColor        string                    `json:"vehicle_color,omitempty"`
	VehiclePlate        string                    `json:"vehicle_plate,omitempty"`
	VehicleYear         *int32                    `json:"vehicle_year,omitempty"`
	ClubMembership      string                    `json:"club_membership,omitempty"`
	MembershipType      string                    `json:"membership_type,omitempty"`
	PlaceOfOrigin       string                    `json:"place_of_origin,omitempty"`
	BirthCountry        string                    `json:"birth_country,omitempty"`
	ParentsWeddingDate  string                    `json:"parents_wedding_date,omitempty"`
	SupermarketClub     string                    `json:"supermarket_club,omitempty"`
	Pet                 string                    `json:"pet,omitempty"`
	TravelCountries     string                    `json:"travel_countries,omitempty"`
	CardBrand           string                    `json:"card_brand,omitempty"`
	CardBank            string                    `json:"card_bank,omitempty"`
	CustomValues        map[string]string         `json:"custom_values"`
	DocumentIdentifiers map[string]string         `json:"document_identifiers"`
	DocumentBadges      []profileDocumentBadge    `json:"document_badges"`
	DocumentPresences   []profileDocumentPresence `json:"document_presences"`
	Version             int64                     `json:"version"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
}

type profilePageResponse struct {
	Profiles []profileResponse `json:"profiles"`
	Page     profilePageMeta   `json:"page"`
}

type profilePageMeta struct {
	Total     int64  `json:"total"`
	Limit     int32  `json:"limit"`
	Offset    int32  `json:"offset"`
	SortField string `json:"sort_field"`
	SortOrder string `json:"sort_order"`
}

type distinctCitiesResponse struct {
	Values []string `json:"values"`
}

func registerProfileRoutes(mux *http.ServeMux, logger *slog.Logger, authentication authenticationService, checker capabilityChecker, service profileService, search searchService, pool listEnrichmentQuerier) {
	mux.HandleFunc("GET /api/v1/profiles", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		limit, parseProblem := parseBoundedInt32(r.URL.Query().Get("limit"), 100, 1, 1000)
		if parseProblem != nil {
			writeProblem(w, r, *parseProblem)
			return
		}
		offset, parseProblem := parseBoundedInt32(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
		if parseProblem != nil {
			writeProblem(w, r, *parseProblem)
			return
		}
		restrict, ids, searchProblem := applySearchQ(r, actor, search, searchdomain.ModuleProfiles)
		if searchProblem != nil {
			writeProblem(w, r, *searchProblem)
			return
		}
		page, err := service.List(r.Context(), actor, profile.ListOptions{
			Limit: limit, Offset: offset, SortField: profile.SortField(r.URL.Query().Get("sort")), SortOrder: profile.SortOrder(r.URL.Query().Get("order")),
			Filters: profile.Filters{FullName: r.URL.Query().Get("full_name"), CPF: r.URL.Query().Get("cpf"), Email: r.URL.Query().Get("email"), City: r.URL.Query().Get("city"), State: r.URL.Query().Get("state"), RestrictIDs: restrict, IDFilter: profileIDsFromSearch(ids)},
		})
		if err != nil {
			writeProfileError(w, r, logger, "list profiles", err)
			return
		}
		response := profilePageResponse{Profiles: make([]profileResponse, 0, len(page.Profiles)), Page: profilePageMeta{Total: page.Total, Limit: page.Limit, Offset: page.Offset, SortField: string(page.SortField), SortOrder: string(page.SortOrder)}}
		reveal := actor.User.Role.CanWriteProfiles()
		for _, value := range page.Profiles {
			response.Profiles = append(response.Profiles, profileFromDomain(value, reveal))
		}
		enrichProfileList(r.Context(), pool, logger, response.Profiles, reveal)
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("GET /api/v1/profiles/cities", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		limit, parseProblem := parseBoundedInt32(r.URL.Query().Get("limit"), 500, 1, 1000)
		if parseProblem != nil {
			writeProblem(w, r, *parseProblem)
			return
		}
		cities, err := service.DistinctCities(r.Context(), actor, profile.Filters{
			FullName: r.URL.Query().Get("full_name"),
			CPF:      r.URL.Query().Get("cpf"),
			Email:    r.URL.Query().Get("email"),
			State:    r.URL.Query().Get("state"),
		}, limit)
		if err != nil {
			writeProfileError(w, r, logger, "list distinct cities", err)
			return
		}
		if cities == nil {
			cities = []string{}
		}
		writeJSON(w, http.StatusOK, distinctCitiesResponse{Values: cities})
	}))
	mux.HandleFunc("POST /api/v1/profiles", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		var request profileValuesRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		created, err := service.Create(r.Context(), actor, request.domainValues(), requestIDFromContext(r.Context()))
		if err != nil {
			writeProfileError(w, r, logger, "create profile", err)
			return
		}
		writeProfileJSON(w, r, logger, pool, created, actor, http.StatusCreated)
	}))
	mux.HandleFunc("GET /api/v1/profiles/{profile_id}", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		id, problem := parseProfileIdentifier(r.PathValue("profile_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		value, err := service.Get(r.Context(), actor, id)
		if err != nil {
			writeProfileError(w, r, logger, "get profile", err)
			return
		}
		writeProfileJSON(w, r, logger, pool, value, actor, http.StatusOK)
	}))
	mux.HandleFunc("PUT /api/v1/profiles/{profile_id}", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		id, problem := parseProfileIdentifier(r.PathValue("profile_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request updateProfileRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		updated, err := service.Update(r.Context(), actor, id, request.Version, request.domainValues(), requestIDFromContext(r.Context()))
		if err != nil {
			writeProfileError(w, r, logger, "update profile", err)
			return
		}
		writeProfileJSON(w, r, logger, pool, updated, actor, http.StatusOK)
	}))
	mux.HandleFunc("POST /api/v1/profiles/{profile_id}/duplicate", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		id, problem := parseProfileIdentifier(r.PathValue("profile_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		duplicated, err := service.Duplicate(r.Context(), actor, id, requestIDFromContext(r.Context()))
		if err != nil {
			writeProfileError(w, r, logger, "duplicate profile", err)
			return
		}
		writeProfileJSON(w, r, logger, pool, duplicated, actor, http.StatusCreated)
	}))
	mux.HandleFunc("DELETE /api/v1/profiles/{profile_id}", requireCapability(auth.CapProfiles, checker, authentication, func(w http.ResponseWriter, r *http.Request) {
		actor, problem := authenticatedSession(r, authentication)
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if service == nil {
			writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "O módulo de pessoas não está configurado"})
			return
		}
		id, problem := parseProfileIdentifier(r.PathValue("profile_id"))
		if problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		var request deleteProfileRequest
		if problem := DecodeJSON(w, r, &request); problem != nil {
			writeProblem(w, r, *problem)
			return
		}
		if err := service.Delete(r.Context(), actor, id, request.Version, request.Confirmation, requestIDFromContext(r.Context())); err != nil {
			writeProfileError(w, r, logger, "delete profile", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

func (request profileValuesRequest) domainValues() profile.Values {
	return profile.Values{
		FullName: request.FullName, SocialName: request.SocialName, CPF: request.CPF, Email: request.Email,
		MobilePhone: request.MobilePhone, LandlinePhone: request.LandlinePhone,
		Address: profile.Address{
			Street: request.Address.Street, Number: request.Address.Number, Complement: request.Address.Complement,
			Neighborhood: request.Address.Neighborhood, City: request.Address.City, State: request.Address.State,
			PostalCode: request.Address.PostalCode,
		},
		Notes: request.Notes, BirthDate: request.BirthDate, Gender: request.Gender, BloodType: request.BloodType,
		Nationality: request.Nationality, BirthCity: request.BirthCity, MaritalStatus: request.MaritalStatus,
		WeddingDate: request.WeddingDate, FatherName: request.FatherName, FatherBirthDate: request.FatherBirthDate,
		MotherName: request.MotherName, MotherBirthDate: request.MotherBirthDate, HealthPlan: request.HealthPlan,
		BloodDonor: request.BloodDonor, OrganDonor: request.OrganDonor, Team: request.Team, Sector: request.Sector,
		Collections: request.Collections, VehicleModel: request.VehicleModel, VehicleColor: request.VehicleColor,
		VehiclePlate: request.VehiclePlate, VehicleYear: request.VehicleYear, ClubMembership: request.ClubMembership,
		MembershipType: request.MembershipType, PlaceOfOrigin: request.PlaceOfOrigin, BirthCountry: request.BirthCountry,
		ParentsWedding: request.ParentsWedding, SupermarketClub: request.SupermarketClub, Pet: request.Pet,
		TravelCountries: request.TravelCountries, CardBrand: request.CardBrand, CardBank: request.CardBank,
	}
}
func (request updateProfileRequest) domainValues() profile.Values {
	return request.profileValuesRequest.domainValues()
}
func writeProfileJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, pool listEnrichmentQuerier, value profile.Profile, actor auth.Session, status int) {
	reveal := actor.User.Role.CanWriteProfiles()
	payload := []profileResponse{profileFromDomain(value, reveal)}
	enrichProfileList(r.Context(), pool, logger, payload, reveal)
	writeJSON(w, status, payload[0])
}

func profileFromDomain(value profile.Profile, reveal bool) profileResponse {
	return profileResponse{
		ID: value.ID.String(), FullName: value.Values.FullName, SocialName: value.Values.SocialName,
		CPF: profile.DisplayCPF(value.Values.CPF, reveal), CPFDigitSum: profile.DigitSumCPF(value.Values.CPF),
		Email: value.Values.Email, MobilePhone: value.Values.MobilePhone, LandlinePhone: value.Values.LandlinePhone,
		Address: profileAddressResponse{
			Street: value.Values.Address.Street, Number: value.Values.Address.Number, Complement: value.Values.Address.Complement,
			Neighborhood: value.Values.Address.Neighborhood, City: value.Values.Address.City, State: value.Values.Address.State,
			PostalCode: value.Values.Address.PostalCode,
		},
		Notes: value.Values.Notes, BirthDate: value.Values.BirthDate, Gender: value.Values.Gender, BloodType: value.Values.BloodType,
		Nationality: value.Values.Nationality, BirthCity: value.Values.BirthCity, MaritalStatus: value.Values.MaritalStatus,
		WeddingDate: value.Values.WeddingDate, FatherName: value.Values.FatherName, FatherBirthDate: value.Values.FatherBirthDate,
		MotherName: value.Values.MotherName, MotherBirthDate: value.Values.MotherBirthDate, HealthPlan: value.Values.HealthPlan,
		BloodDonor: value.Values.BloodDonor, OrganDonor: value.Values.OrganDonor, Team: value.Values.Team, Sector: value.Values.Sector,
		Collections: value.Values.Collections, VehicleModel: value.Values.VehicleModel, VehicleColor: value.Values.VehicleColor,
		VehiclePlate: value.Values.VehiclePlate, VehicleYear: value.Values.VehicleYear, ClubMembership: value.Values.ClubMembership,
		MembershipType: value.Values.MembershipType, PlaceOfOrigin: value.Values.PlaceOfOrigin, BirthCountry: value.Values.BirthCountry,
		ParentsWeddingDate: value.Values.ParentsWedding, SupermarketClub: value.Values.SupermarketClub, Pet: value.Values.Pet,
		TravelCountries: value.Values.TravelCountries, CardBrand: value.Values.CardBrand, CardBank: value.Values.CardBank,
		CustomValues: map[string]string{}, DocumentIdentifiers: map[string]string{},
		DocumentBadges: []profileDocumentBadge{}, DocumentPresences: []profileDocumentPresence{},
		Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
func parseProfileIdentifier(value string) (profile.Identifier, *Problem) {
	id, err := profile.ParseIdentifier(value)
	if err != nil {
		return profile.Identifier{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "O identificador da pessoa é inválido"}
	}
	return id, nil
}
func writeProfileError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	var validation *profile.ValidationError
	switch {
	case errors.As(err, &validation):
		writeProblem(w, r, Problem{Status: http.StatusUnprocessableEntity, Code: ErrorCodeValidation, Message: "Revise os campos informados", FieldErrors: profileFieldProblems(validation)})
	case errors.Is(err, profile.ErrInvalidListOptions):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Filtros, ordenação ou paginação são inválidos"})
	case errors.Is(err, profile.ErrInvalidConfirmation):
		writeProblem(w, r, Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Digite Confirmar para excluir permanentemente"})
	case errors.Is(err, profile.ErrForbidden):
		writeProblem(w, r, Problem{Status: http.StatusForbidden, Code: ErrorCodeForbidden, Message: "Você não possui permissão para esta operação"})
	case errors.Is(err, profile.ErrNotFound):
		writeProblem(w, r, Problem{Status: http.StatusNotFound, Code: ErrorCodeNotFound, Message: "A pessoa não foi encontrada"})
	case errors.Is(err, profile.ErrConflict):
		writeProblem(w, r, Problem{Status: http.StatusConflict, Code: ErrorCodeConflict, Message: "A pessoa foi alterada desde o último carregamento"})
	default:
		logger.Error(operation, "request_id", requestIDFromContext(r.Context()), "error", err)
		writeProblem(w, r, Problem{Status: http.StatusInternalServerError, Code: ErrorCodeInternal, Message: "Não foi possível concluir a operação"})
	}
}
func profileFieldProblems(validation *profile.ValidationError) []FieldProblem {
	problems := make([]FieldProblem, 0, len(validation.Fields))
	for _, field := range validation.Fields {
		problems = append(problems, FieldProblem{Field: field.Field, Code: field.Code, Message: profileFieldMessage(field.Code)})
	}
	return problems
}
func profileFieldMessage(code string) string {
	switch code {
	case "required":
		return "Campo obrigatório"
	case "too_long":
		return "Valor maior que o permitido"
	case "not_mobile":
		return "Informe um celular brasileiro válido"
	case "not_landline":
		return "Informe um telefone fixo brasileiro válido"
	default:
		return "Formato inválido"
	}
}
