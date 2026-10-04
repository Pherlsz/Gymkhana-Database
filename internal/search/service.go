package search

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
)

const (
	defaultRateLimit = 60
	defaultMaxCost   = 500_000
)

var staticFields = []FieldDefinition{
	{Key: "profile.full_name", Module: ModuleProfiles, Label: "Nome completo", Kind: "text"},
	{Key: "profile.social_name", Module: ModuleProfiles, Label: "Nome social", Kind: "text"},
	{Key: "profile.email", Module: ModuleProfiles, Label: "E-mail", Kind: "text"},
	{Key: "profile.mobile_phone", Module: ModuleProfiles, Label: "Celular", Kind: "identifier"},
	{Key: "profile.landline_phone", Module: ModuleProfiles, Label: "Telefone", Kind: "identifier"},
	{Key: "profile.address_street", Module: ModuleProfiles, Label: "Logradouro", Kind: "text"},
	{Key: "profile.address_number", Module: ModuleProfiles, Label: "Número", Kind: "text"},
	{Key: "profile.address_complement", Module: ModuleProfiles, Label: "Complemento", Kind: "text"},
	{Key: "profile.address_neighborhood", Module: ModuleProfiles, Label: "Bairro", Kind: "text"},
	{Key: "profile.address_city", Module: ModuleProfiles, Label: "Cidade", Kind: "text"},
	{Key: "profile.address_state", Module: ModuleProfiles, Label: "UF", Kind: "text"},
	{Key: "profile.address_postal_code", Module: ModuleProfiles, Label: "CEP", Kind: "identifier"},
	{Key: "profile.notes", Module: ModuleProfiles, Label: "Observações", Kind: "long_text"},
	{Key: "profile.team", Module: ModuleProfiles, Label: "Equipe", Kind: "text"},
	{Key: "profile.club_membership", Module: ModuleProfiles, Label: "Sócio clube", Kind: "text"},
	{Key: "profile.place_of_origin", Module: ModuleProfiles, Label: "Naturalidade", Kind: "text"},
	{Key: "profile.birth_country", Module: ModuleProfiles, Label: "País de nascimento", Kind: "text"},
	{Key: "profile.supermarket_club", Module: ModuleProfiles, Label: "Clube de supermercado", Kind: "text"},
	{Key: "profile.pet", Module: ModuleProfiles, Label: "Animal", Kind: "text"},
	{Key: "profile.travel_countries", Module: ModuleProfiles, Label: "Viagem", Kind: "text"},
	{Key: "profile.card_brand", Module: ModuleProfiles, Label: "Bandeira do cartão", Kind: "text"},
	{Key: "profile.card_bank", Module: ModuleProfiles, Label: "Banco do cartão", Kind: "text"},
	{Key: "profile.document_identifier", Module: ModuleProfiles, Label: "Número informado", Kind: "identifier"},
	{Key: "document.type", Module: ModuleDocuments, Label: "Tipo de documento", Kind: "text"},
	{Key: "document.identifier", Module: ModuleDocuments, Label: "Identificador", Kind: "identifier"},
	{Key: "document.date", Module: ModuleDocuments, Label: "Data", Kind: "civil_date"},
	{Key: "document.notes", Module: ModuleDocuments, Label: "Observações", Kind: "long_text"},
	{Key: "document.medium", Module: ModuleDocuments, Label: "Meio", Kind: "text"},
	{Key: "document.current_holder", Module: ModuleDocuments, Label: "Pessoa em uso", Kind: "relation"},
	{Key: "bill.type", Module: ModuleBills, Label: "Tipo de conta/comprovante", Kind: "text"},
	{Key: "bill.printed_holder_name", Module: ModuleBills, Label: "Titular impresso", Kind: "text"},
	{Key: "bill.printed_address", Module: ModuleBills, Label: "Endereço impresso", Kind: "text"},
	{Key: "bill.reference", Module: ModuleBills, Label: "Referência", Kind: "identifier"},
	{Key: "bill.competence", Module: ModuleBills, Label: "Competência", Kind: "civil_month"},
	{Key: "bill.amount", Module: ModuleBills, Label: "Valor", Kind: "decimal"},
	{Key: "bill.currency", Module: ModuleBills, Label: "Moeda", Kind: "text"},
	{Key: "bill.notes", Module: ModuleBills, Label: "Observações", Kind: "long_text"},
	{Key: "bill.medium", Module: ModuleBills, Label: "Meio", Kind: "text"},
	{Key: "bill.current_holder", Module: ModuleBills, Label: "Pessoa em uso", Kind: "relation"},
	{Key: "attachment.filename", Module: ModuleAttachments, Label: "Nome do arquivo", Kind: "text"},
	{Key: "attachment.declared_mime", Module: ModuleAttachments, Label: "Tipo declarado", Kind: "text"},
	{Key: "attachment.detected_mime", Module: ModuleAttachments, Label: "Tipo detectado", Kind: "text"},
	{Key: "attachment.byte_size", Module: ModuleAttachments, Label: "Tamanho em bytes", Kind: "integer"},
	{Key: "attachment.owner_field", Module: ModuleAttachments, Label: "Campo de anexo", Kind: "text"},
}

type Store interface {
	ListDynamicFields(context.Context) ([]FieldDefinition, error)
	ReserveRateLimit(context.Context, auth.Identifier, time.Time, int) error
	Execute(context.Context, Plan) ([]Result, int64, error)
	MatchIDs(context.Context, Plan, Module) ([]string, error)
	MatchProfileHits(context.Context, Plan) ([]ProfileHit, error)
	Suggest(context.Context, SuggestQuery) ([]SuggestHit, error)
}

type ServiceOptions struct {
	Now         func() time.Time
	Timeout     time.Duration
	RateLimit   int
	MaximumCost int
}

type Service struct {
	store       Store
	now         func() time.Time
	timeout     time.Duration
	rateLimit   int
	maximumCost int
}

func NewService(store Store, options ServiceOptions) (*Service, error) {
	if store == nil {
		return nil, ErrInvalidServiceSetup
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Timeout <= 0 {
		options.Timeout = 2 * time.Second
	}
	if options.Timeout > 10*time.Second {
		return nil, ErrInvalidServiceSetup
	}
	if options.RateLimit == 0 {
		options.RateLimit = defaultRateLimit
	}
	if options.RateLimit < 1 || options.RateLimit > 10_000 {
		return nil, ErrInvalidServiceSetup
	}
	if options.MaximumCost == 0 {
		options.MaximumCost = defaultMaxCost
	}
	if options.MaximumCost < 1 {
		return nil, ErrInvalidServiceSetup
	}
	return &Service{
		store:       store,
		now:         options.Now,
		timeout:     options.Timeout,
		rateLimit:   options.RateLimit,
		maximumCost: options.MaximumCost,
	}, nil
}

func (service *Service) Catalog(ctx context.Context, actor auth.Session) (Catalog, error) {
	if !actor.User.Active || !actor.User.Role.CanSearch() {
		return Catalog{}, ErrForbidden
	}
	return service.catalog(ctx, actor.User.Role)
}

func (service *Service) Search(ctx context.Context, actor auth.Session, query Query) (Page, error) {
	normalized, catalog, err := service.prepare(ctx, actor, query)
	if err != nil {
		return Page{}, err
	}
	plans, err := service.plans(normalized, catalog)
	if err != nil {
		return Page{}, err
	}
	fieldCount := selectedFieldCount(normalized, catalog)
	cost := int64(maximum(1, len(normalized.Terms))) * int64(maximum(1, fieldCount)) * int64(normalized.Limit+normalized.Offset) * int64(len(plans))
	if cost > int64(service.maximumCost) {
		return Page{}, ErrCostLimit
	}
	window := service.now().UTC().Truncate(time.Minute)
	if err := service.store.ReserveRateLimit(ctx, actor.User.ID, window, service.rateLimit); err != nil {
		return Page{}, err
	}
	queryContext, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	merged := make([]Result, 0)
	var total int64
	for _, plan := range plans {
		results, count, execErr := service.store.Execute(queryContext, plan)
		if execErr != nil {
			if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(queryContext.Err(), context.DeadlineExceeded) {
				return Page{}, ErrQueryTimeout
			}
			return Page{}, execErr
		}
		total += count
		merged = append(merged, results...)
	}
	if total > MaxResultCardinality {
		return Page{}, ErrCardinalityLimit
	}
	if len(plans) > 1 {
		merged = dedupeResults(merged)
		total = int64(len(merged))
		merged = paginateResults(merged, normalized)
	}
	if !resultsAreAuthorized(merged, normalized, catalog) {
		return Page{}, ErrUnsafeResult
	}
	return Page{
		Results: merged,
		Total:   total,
		Limit:   normalized.Limit,
		Offset:  normalized.Offset,
		Sort:    normalized.Sort,
		Order:   normalized.Order,
	}, nil
}

func (service *Service) MatchIDs(ctx context.Context, actor auth.Session, query Query, grain Module) ([]string, error) {
	if grain == ModuleProfiles {
		query = queryWithLookup(query)
	}
	query.Modules = []Module{grain}
	if grain == ModuleProfiles {
		query.Modules = []Module{ModuleProfiles, ModuleDocuments, ModuleBills}
	}
	normalized, catalog, err := service.prepare(ctx, actor, query)
	if err != nil {
		return nil, err
	}
	plans, err := service.plans(normalized, catalog)
	if err != nil {
		return nil, err
	}
	queryContext, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	seen := map[string]struct{}{}
	ids := make([]string, 0)
	for _, plan := range plans {
		matched, matchErr := service.store.MatchIDs(queryContext, plan, grain)
		if matchErr != nil {
			if errors.Is(matchErr, context.DeadlineExceeded) || errors.Is(queryContext.Err(), context.DeadlineExceeded) {
				return nil, ErrQueryTimeout
			}
			return nil, matchErr
		}
		for _, id := range matched {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if int64(len(ids)) > MaxResultCardinality {
		return nil, ErrCardinalityLimit
	}
	return ids, nil
}

func (service *Service) MatchProfileHits(ctx context.Context, actor auth.Session, query Query) ([]ProfileHit, error) {
	query = queryWithLookup(query)
	query.Modules = []Module{ModuleProfiles, ModuleDocuments, ModuleBills}
	normalized, catalog, err := service.prepare(ctx, actor, query)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(catalog.Fields))
	for _, field := range catalog.Fields {
		allowed[field.Key] = struct{}{}
	}
	plans, err := service.plans(normalized, catalog)
	if err != nil {
		return nil, err
	}
	queryContext, cancel := context.WithTimeout(ctx, service.timeout)
	defer cancel()
	best := map[string]ProfileHit{}
	order := make([]string, 0)
	profiles := map[string]struct{}{}
	for _, plan := range plans {
		matched, matchErr := service.store.MatchProfileHits(queryContext, plan)
		if matchErr != nil {
			if errors.Is(matchErr, context.DeadlineExceeded) || errors.Is(queryContext.Err(), context.DeadlineExceeded) {
				return nil, ErrQueryTimeout
			}
			return nil, matchErr
		}
		for _, hit := range matched {
			if _, ok := allowed[hit.FieldKey]; !ok || hit.ProfileID == "" || hit.FieldKey == "" {
				continue
			}
			profiles[hit.ProfileID] = struct{}{}
			key := hit.ProfileID + "\x00" + hit.FieldKey
			current, seen := best[key]
			if !seen {
				order = append(order, key)
				best[key] = hit
				continue
			}
			if hit.Weight > current.Weight {
				best[key] = hit
			}
		}
	}
	if int64(len(profiles)) > MaxResultCardinality {
		return nil, ErrCardinalityLimit
	}
	hits := make([]ProfileHit, 0, len(order))
	for _, key := range order {
		hits = append(hits, best[key])
	}
	return hits, nil
}

func (service *Service) Suggest(ctx context.Context, actor auth.Session, query SuggestQuery) ([]SuggestHit, error) {
	if !actor.User.Active || !actor.User.Role.CanSearch() {
		return nil, ErrForbidden
	}
	if query.Limit <= 0 || query.Limit > MaxSuggest {
		query.Limit = MaxSuggest
	}
	query.Q = strings.TrimSpace(query.Q)
	if utf8.RuneCountInString(query.Q) > MaxTermLength {
		return nil, &ValidationError{Fields: []FieldError{{Field: "q", Code: "too_long"}}}
	}
	return service.store.Suggest(ctx, query)
}

func queryWithLookup(query Query) Query {
	if strings.TrimSpace(query.Q) == "" {
		return query
	}
	parsed, err := ParseQuery(query.Q)
	if err != nil {
		return query
	}
	simplified := simplifyLookup(parsed)
	query.Q = renderParsed(simplified)
	return query
}

func renderParsed(parsed ParsedQuery) string {
	parts := make([]string, 0)
	for i, branch := range parsed.Branches {
		if i > 0 {
			parts = append(parts, "OU")
		}
		for _, atom := range branch.Atoms {
			text := atom.Value
			if atom.FieldToken != "" {
				text = atom.FieldToken + ":" + atom.Value
			}
			if atom.Exclude {
				text = "-" + text
			}
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func (service *Service) prepare(ctx context.Context, actor auth.Session, query Query) (Query, Catalog, error) {
	if !actor.User.Active || !actor.User.Role.CanSearch() {
		return Query{}, Catalog{}, ErrForbidden
	}
	catalog, err := service.catalog(ctx, actor.User.Role)
	if err != nil {
		return Query{}, Catalog{}, err
	}
	if strings.TrimSpace(query.Q) != "" {
		parsed, parseErr := ParseQuery(query.Q)
		if parseErr != nil {
			return Query{}, Catalog{}, parseErr
		}
		if len(parsed.Modules) > 0 && len(query.Modules) == 0 {
			query.Modules = parsed.Modules
		} else if len(parsed.Modules) > 0 {
			query.Modules = intersectModules(query.Modules, parsed.Modules)
		}
		resolved, resolveErr := resolveParsed(parsed, catalog)
		if resolveErr != nil {
			return Query{}, Catalog{}, resolveErr
		}
		terms := make([]string, 0)
		for _, branch := range resolved.Branches {
			_, _, branchTerms := compileBranch(branch)
			terms = append(terms, branchTerms...)
		}
		query.Terms = uniqueStrings(terms)
	}
	normalized, validation := normalizeQuery(query, catalog)
	if validation != nil {
		return Query{}, Catalog{}, validation
	}
	return normalized, catalog, nil
}

func (service *Service) plans(query Query, catalog Catalog) ([]Plan, error) {
	base := Plan{
		Terms:            query.Terms,
		Modules:          query.Modules,
		Fields:           query.Fields,
		Limit:            query.Limit,
		Offset:           query.Offset,
		Sort:             query.Sort,
		Order:            query.Order,
		StatementTimeout: service.timeout,
		CandidateLimit:   MaxResultCardinality + 1,
	}
	if strings.TrimSpace(query.Q) == "" {
		for _, term := range query.Terms {
			patterns := textSearchPatterns(term)
			base.Includes = append(base.Includes, TermSpec{Term: term, Pattern: patterns[0], Patterns: patterns, FieldKeys: []string{}})
		}
		return []Plan{base}, nil
	}
	parsed, err := ParseQuery(query.Q)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveParsed(parsed, catalog)
	if err != nil {
		return nil, err
	}
	plans := make([]Plan, 0, len(resolved.Branches))
	for _, branch := range resolved.Branches {
		includes, excludes, terms := compileBranch(branch)
		plan := base
		plan.Includes = includes
		plan.Excludes = excludes
		if len(terms) > 0 {
			plan.Terms = terms
		}
		plans = append(plans, plan)
	}
	if len(plans) == 0 {
		return []Plan{base}, nil
	}
	return plans, nil
}

func intersectModules(left, right []Module) []Module {
	allowed := make(map[Module]struct{}, len(right))
	for _, module := range right {
		allowed[module] = struct{}{}
	}
	out := make([]Module, 0, len(left))
	for _, module := range left {
		if _, ok := allowed[module]; ok {
			out = append(out, module)
		}
	}
	return out
}

func dedupeResults(results []Result) []Result {
	seen := make(map[string]int, len(results))
	out := make([]Result, 0, len(results))
	for _, result := range results {
		key := string(result.Module) + ":" + result.EntityID + ":" + result.FieldKey
		if index, ok := seen[key]; ok {
			if result.Score > out[index].Score {
				out[index] = result
			}
			continue
		}
		seen[key] = len(out)
		out = append(out, result)
	}
	return out
}

func paginateResults(results []Result, query Query) []Result {
	start := int(query.Offset)
	if start > len(results) {
		return []Result{}
	}
	end := start + int(query.Limit)
	if end > len(results) {
		end = len(results)
	}
	return results[start:end]
}

func resultsAreAuthorized(results []Result, query Query, catalog Catalog) bool {
	modules := make(map[Module]struct{}, len(query.Modules))
	for _, module := range query.Modules {
		modules[module] = struct{}{}
	}
	fields := make(map[string]Module, len(catalog.Fields))
	for _, field := range catalog.Fields {
		fields[field.Key] = field.Module
	}
	selectedFields := make(map[string]struct{}, len(query.Fields))
	for _, field := range query.Fields {
		selectedFields[field] = struct{}{}
	}
	for _, result := range results {
		if _, ok := modules[result.Module]; !ok {
			return false
		}
		module, ok := fields[result.FieldKey]
		if !ok || module != result.Module {
			return false
		}
		if len(selectedFields) > 0 {
			if _, ok := selectedFields[result.FieldKey]; !ok {
				return false
			}
		}
		if result.EntityID == "" || result.TargetID == "" || result.EntityLabel == "" || result.FieldLabel == "" || utf8.RuneCountInString(result.Preview) > 200 {
			return false
		}
		if !authorizedResultShape(result) {
			return false
		}
	}
	return true
}

func authorizedResultShape(result Result) bool {
	switch result.Module {
	case ModuleProfiles:
		return result.EntityKind == "profile" && result.TargetKind == "profile"
	case ModuleDocuments:
		return result.EntityKind == "document" && result.TargetKind == "document"
	case ModuleBills:
		return result.EntityKind == "bill" && result.TargetKind == "bill"
	case ModuleCustomData:
		return result.EntityKind == result.TargetKind && oneOf(result.TargetKind, "profile", "document", "bill", "custom_entity")
	case ModuleAttachments:
		return result.EntityKind == "attachment" && oneOf(result.TargetKind, "profile", "document", "bill", "custom_entity")
	default:
		return false
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (service *Service) catalog(ctx context.Context, role auth.Role) (Catalog, error) {
	modules := permittedModules(role)
	allowed := make(map[Module]struct{}, len(modules))
	for _, module := range modules {
		allowed[module.Key] = struct{}{}
	}
	fields := make([]FieldDefinition, 0, len(staticFields))
	seenFields := make(map[string]struct{}, len(staticFields))
	for _, field := range staticFields {
		if _, ok := allowed[field.Module]; ok {
			if field.Group == "" {
				field.Group = field.Module
			}
			fields = append(fields, field)
			seenFields[field.Key] = struct{}{}
		}
	}
	dynamic, err := service.store.ListDynamicFields(ctx)
	if err != nil {
		return Catalog{}, err
	}
	for _, field := range dynamic {
		if _, ok := allowed[field.Module]; !ok || !validDynamicField(field) {
			continue
		}
		if field.Group == "" {
			field.Group = ModuleCustomData
		}
		if _, ok := allowed[field.Group]; !ok {
			continue
		}
		if _, duplicate := seenFields[field.Key]; duplicate {
			continue
		}
		fields = append(fields, field)
		seenFields[field.Key] = struct{}{}
	}
	sort.SliceStable(fields, func(left, right int) bool {
		leftGroup, rightGroup := fields[left].Group, fields[right].Group
		if leftGroup == "" {
			leftGroup = fields[left].Module
		}
		if rightGroup == "" {
			rightGroup = fields[right].Module
		}
		if leftGroup != rightGroup {
			return leftGroup < rightGroup
		}
		if fields[left].Label != fields[right].Label {
			return fields[left].Label < fields[right].Label
		}
		return fields[left].Key < fields[right].Key
	})
	return Catalog{
		Modules:   modules,
		Fields:    fields,
		Operators: QueryTokens(),
		Limits: CatalogLimits{
			MaximumTerms:             MaxTerms,
			MaximumTermLength:        MaxTermLength,
			MaximumFields:            MaxFields,
			MaximumPageSize:          MaxPageSize,
			MaximumOffset:            MaxOffset,
			MaximumResultCardinality: MaxResultCardinality,
		},
	}, nil
}

func validDynamicField(field FieldDefinition) bool {
	if field.Module != ModuleCustomData || field.Label == "" {
		return false
	}
	if field.Group != "" && !oneOf(string(field.Group), string(ModuleProfiles), string(ModuleDocuments), string(ModuleBills), string(ModuleCustomData)) {
		return false
	}
	identifier, found := strings.CutPrefix(field.Key, "custom.")
	if !found {
		return false
	}
	if _, err := auth.ParseIdentifier(identifier); err != nil {
		return false
	}
	return oneOf(field.Kind, "text", "long_text", "email", "phone", "integer", "decimal", "boolean", "civil_date", "civil_month", "single_select", "multi_select")
}

func permittedModules(role auth.Role) []ModuleDefinition {
	modules := make([]ModuleDefinition, 0, 5)
	if role.CanReadProfiles() {
		modules = append(modules, ModuleDefinition{Key: ModuleProfiles, Label: "Pessoas"})
	}
	if role.CanReadDocuments() {
		modules = append(modules, ModuleDefinition{Key: ModuleDocuments, Label: "Documentos"})
	}
	if role.CanReadBills() {
		modules = append(modules, ModuleDefinition{Key: ModuleBills, Label: "Contas e comprovantes"})
	}
	if role.CanReadCustomData() {
		modules = append(modules, ModuleDefinition{Key: ModuleCustomData, Label: "Dados personalizados"})
	}
	if role.CanReadAttachments() {
		modules = append(modules, ModuleDefinition{Key: ModuleAttachments, Label: "Anexos"})
	}
	return modules
}

func normalizeQuery(query Query, catalog Catalog) (Query, *ValidationError) {
	validation := &ValidationError{}
	query.Terms = normalizeTerms(query.Terms, validation)
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit < 1 || query.Limit > MaxPageSize {
		validation.add("limit", "out_of_range")
	}
	if query.Offset < 0 || query.Offset > MaxOffset {
		validation.add("offset", "out_of_range")
	}
	if query.Sort == "" {
		query.Sort = SortRelevance
	}
	if query.Order == "" {
		query.Order = SortDescending
	}
	if !query.Sort.Valid() {
		validation.add("sort", "unsupported")
	}
	if !query.Order.Valid() {
		validation.add("order", "unsupported")
	}

	availableModules := make(map[Module]struct{}, len(catalog.Modules))
	for _, module := range catalog.Modules {
		availableModules[module.Key] = struct{}{}
	}
	if len(query.Modules) == 0 {
		for _, module := range catalog.Modules {
			query.Modules = append(query.Modules, module.Key)
		}
	} else {
		query.Modules = uniqueModules(query.Modules, availableModules, validation)
	}
	availableFields := make(map[string]Module, len(catalog.Fields))
	for _, field := range catalog.Fields {
		availableFields[field.Key] = field.Module
	}
	// §13: simple search must not require pre-configured selections. An empty
	// field list searches every authorized field; explicit selections are still
	// capped. Cost stays bounded by the service cost formula.
	if len(query.Fields) > MaxFields {
		validation.add("fields", "too_many")
	} else {
		query.Fields = uniqueFields(query.Fields, availableFields, query.Modules, validation)
	}
	if len(validation.Fields) > 0 {
		return Query{}, validation
	}
	return query, nil
}

func normalizeTerms(terms []string, validation *ValidationError) []string {
	if len(terms) == 0 {
		validation.add("q", "required")
		return nil
	}
	if len(terms) > MaxTerms {
		validation.add("terms", "too_many")
		return nil
	}
	seen := make(map[string]struct{}, len(terms))
	normalized := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.TrimSpace(strings.ToValidUTF8(term, ""))
		if term == "" {
			validation.add("terms", "empty")
			continue
		}
		if utf8.RuneCountInString(term) > MaxTermLength {
			validation.add("terms", "too_long")
			continue
		}
		invalidControl := false
		for _, character := range term {
			if unicode.IsControl(character) {
				invalidControl = true
				break
			}
		}
		if invalidControl {
			validation.add("terms", "invalid_characters")
			continue
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, term)
	}
	if len(normalized) == 0 && len(validation.Fields) == 0 {
		validation.add("terms", "required")
	}
	return normalized
}

func uniqueModules(values []Module, available map[Module]struct{}, validation *ValidationError) []Module {
	seen := make(map[Module]struct{}, len(values))
	modules := make([]Module, 0, len(values))
	for _, module := range values {
		if _, ok := available[module]; !ok {
			validation.add("modules", "unsupported")
			continue
		}
		if _, ok := seen[module]; ok {
			continue
		}
		seen[module] = struct{}{}
		modules = append(modules, module)
	}
	return modules
}

func uniqueFields(values []string, available map[string]Module, modules []Module, validation *ValidationError) []string {
	selectedModules := make(map[Module]struct{}, len(modules))
	for _, module := range modules {
		selectedModules[module] = struct{}{}
	}
	seen := make(map[string]struct{}, len(values))
	fields := make([]string, 0, len(values))
	for _, field := range values {
		module, ok := available[field]
		if !ok {
			validation.add("fields", "unsupported")
			continue
		}
		if _, ok := selectedModules[module]; !ok {
			validation.add("fields", "module_mismatch")
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		fields = append(fields, field)
	}
	return fields
}

func selectedFieldCount(query Query, catalog Catalog) int {
	if len(query.Fields) > 0 {
		return len(query.Fields)
	}
	modules := make(map[Module]struct{}, len(query.Modules))
	for _, module := range query.Modules {
		modules[module] = struct{}{}
	}
	count := 0
	for _, field := range catalog.Fields {
		if _, ok := modules[field.Module]; ok {
			count++
		}
	}
	return count
}

func textSearchPatterns(term string) []string {
	patterns := []string{"%" + escapeLike(term) + "%"}
	if folded := foldValue(term); folded != "" {
		patterns = append(patterns, "%"+escapeLike(folded)+"%")
	}
	return uniqueStrings(patterns)
}

func (validation *ValidationError) add(field, code string) {
	validation.Fields = append(validation.Fields, FieldError{Field: field, Code: code})
}

func maximum(left, right int) int {
	if left > right {
		return left
	}
	return right
}
