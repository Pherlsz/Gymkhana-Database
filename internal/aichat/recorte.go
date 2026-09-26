package aichat

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	querydomain "github.com/Pherlsz/Gymkhana-Database/internal/queryengine"
)

// TableRecorte is a query reference compiled into the grid's own filters and
// columns. The link carries this, not result rows and not the stored plan.
//
// ponytail: only operators the sheet already applies the same way (contains on
// text, eq on UF/meio/competência). Relations, groups, patterns and aggregates
// do not fit; those stay in the chat. Upgrade path is a sheet WHERE built from
// the query plan.
type TableRecorte struct {
	Table   string            `json:"table"`
	Filters map[string]string `json:"filters"`
	Columns []string          `json:"columns"`
	Sort    string            `json:"sort,omitempty"`
	Order   string            `json:"order,omitempty"`
}

type gridField struct {
	param string
	op    querydomain.Operator
}

func CompileTableRecorte(kind ResultReferenceKind, logical []byte) (TableRecorte, error) {
	if kind != ResultReferenceQuery {
		return TableRecorte{}, ErrInvalidInput
	}
	var stored storedQueryRequest
	if err := json.Unmarshal(logical, &stored); err != nil {
		return TableRecorte{}, ErrInvalidInput
	}
	plan := stored.Plan
	if plan.Version != querydomain.PlanVersionV1 || plan.Filter == nil ||
		len(plan.GroupBy) > 0 || len(plan.Aggregates) > 0 || plan.Having != nil ||
		len(plan.Patterns) > 0 || plan.Set != nil || plan.Combination != nil {
		return TableRecorte{}, ErrInvalidInput
	}
	filters, columns, sorts, table, ok := gridCatalog(plan.RootEntity)
	if !ok {
		return TableRecorte{}, ErrInvalidInput
	}
	nodes, err := gridPredicates(plan.Filter, 0)
	if err != nil || len(nodes) == 0 {
		return TableRecorte{}, ErrInvalidInput
	}
	compiled := TableRecorte{Table: table, Filters: map[string]string{}, Columns: []string{}}
	seen := map[string]bool{}
	for _, node := range nodes {
		binding, known := filters[node.Field]
		if !known || node.Operator != binding.op || len(node.Values) != 1 {
			return TableRecorte{}, ErrInvalidInput
		}
		if seen[binding.param] {
			return TableRecorte{}, ErrInvalidInput
		}
		value, ok := gridValue(binding.param, node.Values[0])
		if !ok {
			return TableRecorte{}, ErrInvalidInput
		}
		seen[binding.param] = true
		compiled.Filters[binding.param] = value
	}
	seenColumn := map[string]bool{}
	for _, field := range plan.Projections {
		key, known := columns[field]
		if !known || seenColumn[key] {
			continue
		}
		seenColumn[key] = true
		compiled.Columns = append(compiled.Columns, key)
	}
	if len(plan.Sort) == 1 {
		if key, known := sorts[plan.Sort[0].Field]; known &&
			(plan.Sort[0].Direction == querydomain.SortAscending || plan.Sort[0].Direction == querydomain.SortDescending) {
			compiled.Sort = key
			compiled.Order = string(plan.Sort[0].Direction)
		}
	}
	return compiled, nil
}

func gridPredicates(node *querydomain.FilterNode, depth int) ([]querydomain.FilterNode, error) {
	if node == nil || depth > querydomain.MaximumFilterDepth {
		return nil, ErrInvalidInput
	}
	switch node.Kind {
	case querydomain.FilterPredicate:
		return []querydomain.FilterNode{*node}, nil
	case querydomain.FilterGroup:
		if node.Conjunction == querydomain.ConjunctionOr || len(node.Children) == 0 {
			return nil, ErrInvalidInput
		}
		var nodes []querydomain.FilterNode
		for _, child := range node.Children {
			part, err := gridPredicates(&child, depth+1)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, part...)
		}
		return nodes, nil
	default:
		return nil, ErrInvalidInput
	}
}

func gridValue(param, raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" || utf8.RuneCountInString(value) > 120 || strings.ContainsAny(value, "\r\n") {
		return "", false
	}
	switch param {
	case "state":
		value = strings.ToUpper(value)
		if len(value) != 2 {
			return "", false
		}
	case "document_medium", "bill_medium":
		value = strings.ToUpper(value)
		if value != "PHYSICAL" && value != "DIGITAL" {
			return "", false
		}
	}
	return value, true
}

func gridCatalog(root string) (map[string]gridField, map[string]string, map[string]string, string, bool) {
	switch root {
	case "profiles":
		return map[string]gridField{
				"profile.full_name":     {param: "full_name", op: querydomain.OperatorContains},
				"profile.email":         {param: "email", op: querydomain.OperatorContains},
				"profile.cpf":           {param: "cpf", op: querydomain.OperatorContains},
				"profile.address_city":  {param: "city", op: querydomain.OperatorContains},
				"profile.address_state": {param: "state", op: querydomain.OperatorEqual},
			}, map[string]string{
				"profile.full_name": "full_name", "profile.social_name": "social_name", "profile.gender": "gender",
				"profile.birth_date": "birth_date", "profile.blood_type": "blood_type", "profile.marital_status": "marital_status",
				"profile.nationality": "nationality", "profile.birth_city": "birth_city", "profile.birth_country": "birth_country",
				"profile.place_of_origin": "place_of_origin", "profile.cpf": "cpf", "profile.email": "email",
				"profile.mobile_phone": "mobile", "profile.landline_phone": "landline", "profile.address_street": "street",
				"profile.address_number": "number", "profile.address_complement": "complement",
				"profile.address_neighborhood": "neighborhood", "profile.address_city": "city", "profile.address_state": "state",
				"profile.address_postal_code": "postal_code", "profile.father_name": "father_name",
				"profile.father_birth_date": "father_birth_date", "profile.mother_name": "mother_name",
				"profile.mother_birth_date": "mother_birth_date", "profile.wedding_date": "wedding_date",
				"profile.parents_wedding_date": "parents_wedding_date", "profile.team": "team", "profile.sector": "sector",
				"profile.club_membership": "club_membership", "profile.membership_type": "membership_type",
				"profile.collections": "collections", "profile.vehicle_model": "vehicle_model",
				"profile.vehicle_color": "vehicle_color", "profile.vehicle_plate": "vehicle_plate",
				"profile.vehicle_year": "vehicle_year", "profile.health_plan": "health_plan",
				"profile.blood_donor": "blood_donor", "profile.organ_donor": "organ_donor",
			}, map[string]string{
				"profile.full_name": "full_name", "profile.cpf": "cpf", "profile.email": "email",
				"profile.address_city": "address_city", "profile.address_street": "address_street",
				"profile.address_neighborhood": "address_neighborhood", "profile.mobile_phone": "mobile_phone",
				"profile.birth_date": "birth_date", "profile.updated_at": "updated_at",
			}, "people", true
	case "documents":
		return map[string]gridField{
				"document.identifier": {param: "document_identifier", op: querydomain.OperatorContains},
				"document.medium":     {param: "document_medium", op: querydomain.OperatorEqual},
			}, map[string]string{
				"document.identifier": "identifier", "document.type": "type", "document.date": "date",
				"document.valid_until": "valid_until", "document.notes": "notes", "document.owner_name": "owner",
				"document.medium": "medium", "document.idle_custody": "idle_custody",
				"document.current_holder_name": "current_holder",
			}, map[string]string{
				"document.identifier": "identifier_value", "document.type": "type_label",
				"document.date": "document_date", "document.updated_at": "updated_at",
			}, "documents", true
	case "bills":
		return map[string]gridField{
				"bill.reference":  {param: "bill_reference", op: querydomain.OperatorContains},
				"bill.competence": {param: "bill_competence", op: querydomain.OperatorEqual},
				"bill.medium":     {param: "bill_medium", op: querydomain.OperatorEqual},
			}, map[string]string{
				"bill.reference": "reference", "bill.type": "type", "bill.competence": "competence",
				"bill.amount": "amount", "bill.currency": "currency", "bill.notes": "notes",
				"bill.owner_name": "owner", "bill.printed_holder_name": "printed_holder_name",
				"bill.printed_address": "printed_address", "bill.medium": "medium",
				"bill.idle_custody": "idle_custody",
			}, map[string]string{
				"bill.reference": "reference_value", "bill.type": "type_label",
				"bill.competence": "competence", "bill.amount": "amount", "bill.updated_at": "updated_at",
			}, "bills", true
	default:
		return nil, nil, nil, "", false
	}
}
