package httpserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/Pherlsz/Gymkhana-Database/internal/customdata"
)

func customActor(w http.ResponseWriter, r *http.Request, authentication authenticationService, service customDataService) (auth.Session, bool) {
	if service == nil {
		writeProblem(w, r, Problem{Status: http.StatusServiceUnavailable, Code: ErrorCodeInternal, Message: "Custom data is unavailable"})
		return auth.Session{}, false
	}
	actor, problem := authenticatedSession(r, authentication)
	if problem != nil {
		writeProblem(w, r, *problem)
		return auth.Session{}, false
	}
	return actor, true
}

func parseCustomID(value string) (customdata.Identifier, *Problem) {
	id, err := customdata.ParseIdentifier(value)
	if err != nil {
		return customdata.Identifier{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Custom data identifier is invalid"}
	}
	return id, nil
}

func parseOptionalCustomID(value string) (*customdata.Identifier, *Problem) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	id, problem := parseCustomID(value)
	if problem != nil {
		return nil, problem
	}
	return &id, nil
}

func parseOptionalTargetID(kind customdata.TargetKind, value string) (customdata.Identifier, *Problem) {
	if kind == customdata.TargetProfile && strings.TrimSpace(value) == "" {
		return customdata.Identifier{}, nil
	}
	return parseCustomID(value)
}

func parseValueTarget(kindValue, idValue string) (customdata.TargetReference, *Problem) {
	kind := customdata.ValueTargetKind(strings.ToUpper(strings.ReplaceAll(kindValue, "-", "_")))
	id, problem := parseCustomID(idValue)
	if problem != nil {
		return customdata.TargetReference{}, problem
	}
	target := customdata.TargetReference{Kind: kind, ID: id}
	if !target.Valid() {
		return customdata.TargetReference{}, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Custom data target is invalid"}
	}
	return target, nil
}

func parseCustomBool(value string) (*bool, *Problem) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, &Problem{Status: http.StatusBadRequest, Code: ErrorCodeBadRequest, Message: "Boolean filter is invalid"}
	}
	return &parsed, nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (request customEntityTypeRequest) domain() customdata.EntityTypeValues {
	return customdata.EntityTypeValues{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, ProfileCardinality: request.ProfileCardinality}
}

func (request customFieldRequest) domain() (customdata.FieldDefinitionValues, *Problem) {
	targetID, problem := parseOptionalTargetID(request.TargetKind, request.TargetID)
	if problem != nil {
		return customdata.FieldDefinitionValues{}, problem
	}
	return customdata.FieldDefinitionValues{TargetKind: request.TargetKind, TargetID: targetID, TechnicalKey: request.TechnicalKey, Label: request.Label, Kind: request.FieldKind, Required: request.Required, Active: request.Active, MinimumLength: request.MinimumLength, MaximumLength: request.MaximumLength, ValidationRegex: request.ValidationRegex, MinimumDecimal: request.MinimumDecimal, MaximumDecimal: request.MaximumDecimal}, nil
}

func (request customOptionRequest) domain() customdata.OptionValues {
	return customdata.OptionValues{TechnicalKey: request.TechnicalKey, Label: request.Label, Active: request.Active, SortOrder: request.SortOrder}
}

func customValueInputs(values []customValueRequest) ([]customdata.ValueInput, *Problem) {
	result := make([]customdata.ValueInput, 0, len(values))
	for _, value := range values {
		id, problem := parseCustomID(value.FieldDefinitionID)
		if problem != nil {
			return nil, problem
		}
		input := customdata.ValueInput{FieldDefinitionID: id, Kind: value.FieldKind, Text: value.Text, Integer: value.Integer, Decimal: value.Decimal, Boolean: value.Boolean, CivilDate: value.CivilDate, CivilMonth: value.CivilMonth}
		for _, raw := range value.OptionIDs {
			optionID, problem := parseCustomID(raw)
			if problem != nil {
				return nil, problem
			}
			input.OptionIDs = append(input.OptionIDs, optionID)
		}
		result = append(result, input)
	}
	return result, nil
}
