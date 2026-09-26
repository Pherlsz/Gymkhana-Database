package document

type Claim string

const (
	ClaimAbsence        Claim = "absence"
	ClaimIndication     Claim = "indication"
	ClaimInformedNumber Claim = "informed_number"
)

func (claim Claim) Valid() bool {
	return claim == ClaimAbsence || claim == ClaimIndication || claim == ClaimInformedNumber
}

type IdleCustody string

const (
	IdleCustodyOrganization IdleCustody = "ORGANIZATION"
	IdleCustodyOwner        IdleCustody = "OWNER"
)

func (custody IdleCustody) Valid() bool {
	return custody == IdleCustodyOrganization || custody == IdleCustodyOwner
}

type BadgeKind string

const (
	BadgeIndication               BadgeKind = "indication"
	BadgeInformedNumber           BadgeKind = "informed_number"
	BadgePhysical                 BadgeKind = "physical"
	BadgePhysicalWithOwner        BadgeKind = "physical_with_owner"
	BadgeDigital                  BadgeKind = "digital"
	BadgePhysicalDigital          BadgeKind = "physical_digital"
	BadgePhysicalWithOwnerDigital BadgeKind = "physical_with_owner_digital"
)

type PresenceState struct {
	Claim       Claim
	Identifier  string
	HasPhysical bool
	HasDigital  bool
	IdleCustody IdleCustody
	InUse       bool
}

func (state PresenceState) Badge() BadgeKind {
	if state.HasPhysical && state.HasDigital {
		if state.IdleCustody == IdleCustodyOwner {
			return BadgePhysicalWithOwnerDigital
		}
		return BadgePhysicalDigital
	}
	if state.HasPhysical {
		if state.IdleCustody == IdleCustodyOwner {
			return BadgePhysicalWithOwner
		}
		return BadgePhysical
	}
	if state.HasDigital {
		return BadgeDigital
	}
	switch state.Claim {
	case ClaimIndication:
		return BadgeIndication
	case ClaimInformedNumber:
		return BadgeInformedNumber
	default:
		return ""
	}
}

func (state PresenceState) InHands() bool {
	if !state.HasPhysical {
		return false
	}
	return state.IdleCustody == IdleCustodyOrganization || state.InUse
}

func PresenceWrite(identifier string, existing *PresenceState, keepExistingNumber bool) (Claim, string) {
	if identifier != "" {
		return ClaimInformedNumber, identifier
	}
	if keepExistingNumber && existing != nil && existing.Claim == ClaimInformedNumber && existing.Identifier != "" {
		return ClaimInformedNumber, existing.Identifier
	}
	return ClaimIndication, ""
}

func PresenceClaimWrite(claim Claim, identifier string, hasExemplar bool) (Claim, string, error) {
	if !claim.Valid() {
		return "", "", &ValidationError{Fields: []FieldError{{Field: "claim", Code: "invalid_value"}}}
	}
	if claim == ClaimAbsence {
		if hasExemplar {
			return "", "", &ValidationError{Fields: []FieldError{{Field: "claim", Code: "has_exemplar"}}}
		}
		if identifier != "" {
			return "", "", &ValidationError{Fields: []FieldError{{Field: "identifier_value", Code: "unexpected"}}}
		}
		return ClaimAbsence, "", nil
	}
	if claim == ClaimIndication {
		if hasExemplar {
			return "", "", &ValidationError{Fields: []FieldError{{Field: "claim", Code: "has_exemplar"}}}
		}
		if identifier != "" {
			return "", "", &ValidationError{Fields: []FieldError{{Field: "identifier_value", Code: "unexpected"}}}
		}
		return ClaimIndication, "", nil
	}
	if identifier == "" {
		return "", "", &ValidationError{Fields: []FieldError{{Field: "identifier_value", Code: "required"}}}
	}
	return ClaimInformedNumber, identifier, nil
}
