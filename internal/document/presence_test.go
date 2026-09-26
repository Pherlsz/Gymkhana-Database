package document

import "testing"

func TestBadgeKindPositiveStates(t *testing.T) {
	cases := []struct {
		name  string
		state PresenceState
		want  BadgeKind
	}{
		{name: "unspecified", state: PresenceState{}, want: ""},
		{name: "absence", state: PresenceState{Claim: ClaimAbsence}, want: ""},
		{name: "indication", state: PresenceState{Claim: ClaimIndication}, want: BadgeIndication},
		{name: "informed number", state: PresenceState{Claim: ClaimInformedNumber, Identifier: "123"}, want: BadgeInformedNumber},
		{name: "physical organization", state: PresenceState{HasPhysical: true, IdleCustody: IdleCustodyOrganization}, want: BadgePhysical},
		{name: "physical with owner", state: PresenceState{HasPhysical: true, IdleCustody: IdleCustodyOwner}, want: BadgePhysicalWithOwner},
		{name: "digital", state: PresenceState{HasDigital: true}, want: BadgeDigital},
		{name: "physical and digital", state: PresenceState{HasPhysical: true, HasDigital: true, IdleCustody: IdleCustodyOrganization}, want: BadgePhysicalDigital},
		{name: "physical with owner and digital", state: PresenceState{HasPhysical: true, HasDigital: true, IdleCustody: IdleCustodyOwner}, want: BadgePhysicalWithOwnerDigital},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := test.state.Badge(); got != test.want {
				t.Fatalf("badge = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInHandsCountsOrganizationOrCurrentUse(t *testing.T) {
	if (PresenceState{HasPhysical: true, IdleCustody: IdleCustodyOrganization}).InHands() != true {
		t.Fatal("organization physical must count as in hands")
	}
	if (PresenceState{HasPhysical: true, IdleCustody: IdleCustodyOwner, InUse: true}).InHands() != true {
		t.Fatal("physical currently in use must count as in hands")
	}
	if (PresenceState{HasPhysical: true, IdleCustody: IdleCustodyOwner}).InHands() {
		t.Fatal("physical resting with the owner must not count as inventory in hands")
	}
	if (PresenceState{HasDigital: true}).InHands() {
		t.Fatal("digital exemplar must not count as inventory in hands")
	}
}

func TestPresenceClaimWriteAbsenceAndIndication(t *testing.T) {
	claim, identifier, err := PresenceClaimWrite(ClaimAbsence, "", false)
	if err != nil || claim != ClaimAbsence || identifier != "" {
		t.Fatalf("absence = %#v %#v %v", claim, identifier, err)
	}
	if _, _, err := PresenceClaimWrite(ClaimAbsence, "", true); err == nil {
		t.Fatal("absence with exemplar must fail")
	}
	if _, _, err := PresenceClaimWrite(ClaimIndication, "", true); err == nil {
		t.Fatal("indication with exemplar must fail")
	}
	claim, identifier, err = PresenceClaimWrite(ClaimInformedNumber, "52998224725", true)
	if err != nil || claim != ClaimInformedNumber || identifier != "52998224725" {
		t.Fatalf("informed number = %#v %#v %v", claim, identifier, err)
	}
}

func TestPresenceWriteKeepsNumberOnCreateAndClearsOnUpdate(t *testing.T) {
	existing := &PresenceState{Claim: ClaimInformedNumber, Identifier: "52998224725"}
	claim, identifier := PresenceWrite("", existing, true)
	if claim != ClaimInformedNumber || identifier != "52998224725" {
		t.Fatalf("create without number kept %#v %#v", claim, identifier)
	}
	claim, identifier = PresenceWrite("", existing, false)
	if claim != ClaimIndication || identifier != "" {
		t.Fatalf("clearing number must become indication: %#v %#v", claim, identifier)
	}
	claim, identifier = PresenceWrite("12345678901", existing, false)
	if claim != ClaimInformedNumber || identifier != "12345678901" {
		t.Fatalf("informed number write = %#v %#v", claim, identifier)
	}
}

func TestOperationalStatusOwnerIdleIsNotAvailable(t *testing.T) {
	if OperationalStatus(MediumPhysical, false, IdleCustodyOrganization) != StatusAvailable {
		t.Fatal("organization idle physical must be available")
	}
	if OperationalStatus(MediumPhysical, false, IdleCustodyOwner) != "" {
		t.Fatal("owner idle physical must not appear as available inventory")
	}
	if OperationalStatus(MediumPhysical, true, IdleCustodyOwner) != StatusInUse {
		t.Fatal("in-use physical must stay in use regardless of idle custody")
	}
	if OperationalStatus(MediumDigital, true, "") != "" {
		t.Fatal("digital must not expose lending status")
	}
}
