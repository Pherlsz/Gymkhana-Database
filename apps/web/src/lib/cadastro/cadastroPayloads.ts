import type { Profile, ProfileValuesRequest } from "../api/client";
import type { PersonComplementaryState } from "./components/PersonComplementaryGroup";
import type { PersonDemographicsState } from "./components/PersonDemographicsGroup";
import type { PersonFamilyState } from "./components/PersonFamilyGroup";

export function formatAddressLine(demographics: PersonDemographicsState): string {
  const streetLine = [demographics.street.trim(), demographics.number.trim()]
    .filter(Boolean)
    .join(", ");
  const cityLine = [demographics.city.trim(), demographics.state.trim()].filter(Boolean).join("/");
  return [streetLine, demographics.complement.trim(), demographics.neighborhood.trim(), cityLine]
    .filter(Boolean)
    .join(" — ");
}

export function buildProfilePayload(
  demographics: PersonDemographicsState,
  family: PersonFamilyState,
  complementary: PersonComplementaryState,
  fallbackName: string,
  defaultHolder: string,
  notes = "",
): ProfileValuesRequest {
  const finalName = demographics.fullName.trim() || fallbackName.trim() || defaultHolder;
  const vehicleYearParsed = parseInt(complementary.vehicleYear.trim(), 10);

  return {
    full_name: finalName,
    social_name: demographics.socialName.trim(),
    cpf: demographics.cpf.trim(),
    email: demographics.email.trim(),
    mobile_phone: demographics.phone.trim(),
    landline_phone: demographics.landline.trim(),
    ...(demographics.birthDate ? { birth_date: demographics.birthDate } : {}),
    ...(demographics.gender ? { gender: demographics.gender } : {}),
    ...(demographics.maritalStatus ? { marital_status: demographics.maritalStatus } : {}),
    ...(demographics.bloodType ? { blood_type: demographics.bloodType } : {}),
    ...(demographics.nationality.trim() ? { nationality: demographics.nationality.trim() } : {}),
    ...(demographics.birthCity.trim() ? { birth_city: demographics.birthCity.trim() } : {}),
    ...(demographics.birthCountry.trim()
      ? { birth_country: demographics.birthCountry.trim() }
      : {}),
    ...(demographics.placeOfOrigin.trim()
      ? { place_of_origin: demographics.placeOfOrigin.trim() }
      : {}),
    ...(family.fatherName.trim() ? { father_name: family.fatherName.trim() } : {}),
    ...(family.fatherBirthDate ? { father_birth_date: family.fatherBirthDate } : {}),
    ...(family.motherName.trim() ? { mother_name: family.motherName.trim() } : {}),
    ...(family.motherBirthDate ? { mother_birth_date: family.motherBirthDate } : {}),
    ...(family.weddingDate ? { wedding_date: family.weddingDate } : {}),
    ...(family.parentsWeddingDate ? { parents_wedding_date: family.parentsWeddingDate } : {}),
    ...(complementary.vehicleModel.trim()
      ? { vehicle_model: complementary.vehicleModel.trim() }
      : {}),
    ...(complementary.vehicleColor.trim()
      ? { vehicle_color: complementary.vehicleColor.trim() }
      : {}),
    ...(complementary.vehiclePlate.trim()
      ? { vehicle_plate: complementary.vehiclePlate.trim() }
      : {}),
    ...(Number.isFinite(vehicleYearParsed) && vehicleYearParsed > 0
      ? { vehicle_year: vehicleYearParsed }
      : {}),
    ...(complementary.healthPlan.trim() ? { health_plan: complementary.healthPlan.trim() } : {}),
    ...(typeof complementary.bloodDonor === "boolean"
      ? { blood_donor: complementary.bloodDonor }
      : {}),
    ...(typeof complementary.organDonor === "boolean"
      ? { organ_donor: complementary.organDonor }
      : {}),
    ...(complementary.team.trim() ? { team: complementary.team.trim() } : {}),
    ...(complementary.sector.trim() ? { sector: complementary.sector.trim() } : {}),
    ...(complementary.clubMembership.trim()
      ? { club_membership: complementary.clubMembership.trim() }
      : {}),
    ...(complementary.membershipType.trim()
      ? { membership_type: complementary.membershipType.trim() }
      : {}),
    ...(complementary.collections.trim() ? { collections: complementary.collections.trim() } : {}),
    ...(complementary.pet.trim() ? { pet: complementary.pet.trim() } : {}),
    ...(complementary.supermarketClub.trim()
      ? { supermarket_club: complementary.supermarketClub.trim() }
      : {}),
    ...(complementary.travelCountries.trim()
      ? { travel_countries: complementary.travelCountries.trim() }
      : {}),
    ...(complementary.cardBrand.trim() ? { card_brand: complementary.cardBrand.trim() } : {}),
    ...(complementary.cardBank.trim() ? { card_bank: complementary.cardBank.trim() } : {}),
    address: {
      street: demographics.street.trim(),
      number: demographics.number.trim(),
      complement: demographics.complement.trim(),
      neighborhood: demographics.neighborhood.trim(),
      city: demographics.city.trim(),
      state: demographics.state.trim(),
      postal_code: demographics.postalCode.trim(),
    },
    notes: notes.trim(),
  };
}

export function demographicsHasMoreDetails(state: PersonDemographicsState): boolean {
  return Boolean(
    state.socialName.trim() ||
    state.landline.trim() ||
    state.gender ||
    state.maritalStatus ||
    state.bloodType ||
    state.nationality.trim() ||
    state.birthCity.trim() ||
    state.birthCountry.trim() ||
    state.placeOfOrigin.trim(),
  );
}

export function familyHasValues(family: PersonFamilyState): boolean {
  return Boolean(
    family.fatherName.trim() ||
    family.fatherBirthDate ||
    family.motherName.trim() ||
    family.motherBirthDate ||
    family.weddingDate ||
    family.parentsWeddingDate,
  );
}

export function complementaryHasValues(complementary: PersonComplementaryState): boolean {
  return Boolean(
    complementary.vehicleModel.trim() ||
    complementary.vehicleColor.trim() ||
    complementary.vehiclePlate.trim() ||
    complementary.vehicleYear.trim() ||
    complementary.healthPlan.trim() ||
    complementary.bloodDonor != null ||
    complementary.organDonor != null ||
    complementary.team.trim() ||
    complementary.sector.trim() ||
    complementary.clubMembership.trim() ||
    complementary.membershipType.trim() ||
    complementary.collections.trim() ||
    complementary.pet.trim() ||
    complementary.supermarketClub.trim() ||
    complementary.travelCountries.trim() ||
    complementary.cardBrand.trim() ||
    complementary.cardBank.trim(),
  );
}

export function mapProfileToState(p: Profile): {
  demographics: PersonDemographicsState;
  family: PersonFamilyState;
  complementary: PersonComplementaryState;
} {
  return {
    demographics: {
      fullName: p.full_name,
      socialName: p.social_name || "",
      cpf: p.cpf || "",
      birthDate: p.birth_date || undefined,
      email: p.email || "",
      phone: p.mobile_phone || "",
      landline: p.landline_phone || "",
      street: p.address?.street || "",
      number: p.address?.number || "",
      complement: p.address?.complement || "",
      neighborhood: p.address?.neighborhood || "",
      city: p.address?.city || "",
      state: p.address?.state || "",
      postalCode: p.address?.postal_code || "",
      gender: p.gender || undefined,
      maritalStatus: p.marital_status || undefined,
      bloodType: p.blood_type || undefined,
      nationality: p.nationality || "",
      birthCity: p.birth_city || "",
      birthCountry: p.birth_country || "",
      placeOfOrigin: p.place_of_origin || "",
    },
    family: {
      fatherName: p.father_name || "",
      fatherBirthDate: p.father_birth_date || undefined,
      motherName: p.mother_name || "",
      motherBirthDate: p.mother_birth_date || undefined,
      weddingDate: p.wedding_date || undefined,
      parentsWeddingDate: p.parents_wedding_date || undefined,
    },
    complementary: {
      vehicleModel: p.vehicle_model || "",
      vehicleColor: p.vehicle_color || "",
      vehiclePlate: p.vehicle_plate || "",
      vehicleYear: p.vehicle_year ? String(p.vehicle_year) : "",
      healthPlan: p.health_plan || "",
      bloodDonor: p.blood_donor !== undefined ? p.blood_donor : undefined,
      organDonor: p.organ_donor !== undefined ? p.organ_donor : undefined,
      team: p.team || "",
      sector: p.sector || "",
      clubMembership: p.club_membership || "",
      membershipType: p.membership_type || "",
      collections: p.collections || "",
      pet: p.pet || "",
      supermarketClub: p.supermarket_club || "",
      travelCountries: p.travel_countries || "",
      cardBrand: p.card_brand || "",
      cardBank: p.card_bank || "",
    },
  };
}
