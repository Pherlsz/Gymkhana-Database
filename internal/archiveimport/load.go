package archiveimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/profile"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DocumentType struct {
	ID               pgtype.UUID
	TechnicalKey     string
	UniquenessPolicy string
}

type Report struct {
	Read         int            `json:"read"`
	Inserted     int            `json:"inserted"`
	Skipped      int            `json:"skipped"`
	Presences    int            `json:"presences"`
	SkipReasons  map[string]int `json:"skip_reasons,omitempty"`
	Dropped      map[string]int `json:"dropped_fields,omitempty"`
	UnknownTypes map[string]int `json:"unknown_types,omitempty"`
}

func GuardDevProject(ctx context.Context, pool *pgxpool.Pool, replace bool) error {
	var projectID string
	if err := pool.QueryRow(ctx, `SELECT current_setting('neon.project_id', true)`).Scan(&projectID); err != nil {
		return fmt.Errorf("read neon.project_id: %w", err)
	}
	if projectID != requiredDevProjectID {
		return fmt.Errorf("refusing import: neon.project_id=%q, want Dev-18 %s", projectID, requiredDevProjectID)
	}
	var profiles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM profiles`).Scan(&profiles); err != nil {
		return fmt.Errorf("count profiles: %w", err)
	}
	if profiles == 0 {
		return nil
	}
	if !replace {
		return fmt.Errorf("refusing import: Dev-18 already has %d profiles", profiles)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM custom_field_values WHERE profile_id IS NOT NULL`); err != nil {
		return fmt.Errorf("clear custom field values: %w", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM document_presences`); err != nil {
		return fmt.Errorf("clear document presences: %w", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM profiles`); err != nil {
		return fmt.Errorf("clear profiles: %w", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM app_metadata WHERE key = 'archive.import.dev18'`); err != nil {
		return fmt.Errorf("clear archive metadata: %w", err)
	}
	return nil
}

func LoadTypes(ctx context.Context, pool *pgxpool.Pool) (map[string]DocumentType, error) {
	rows, err := pool.Query(ctx, `SELECT id, technical_key, uniqueness_policy FROM document_types`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	types := map[string]DocumentType{}
	for rows.Next() {
		var item DocumentType
		if err := rows.Scan(&item.ID, &item.TechnicalKey, &item.UniquenessPolicy); err != nil {
			return nil, err
		}
		types[item.TechnicalKey] = item
	}
	return types, rows.Err()
}

func InsertBatch(ctx context.Context, tx pgx.Tx, types map[string]DocumentType, records []Record) (inserted, presences int, unknown map[string]int, err error) {
	unknown = map[string]int{}
	profileRows := make([][]any, 0, len(records))
	presenceRows := make([][]any, 0, len(records)*4)
	for _, record := range records {
		id, err := profile.NewIdentifier()
		if err != nil {
			return 0, 0, unknown, err
		}
		params := profileCreateRow(id, record.Values)
		profileRows = append(profileRows, params)
		seen := map[string]bool{}
		for _, item := range record.Presences {
			if seen[item.TypeKey] {
				continue
			}
			docType, ok := types[item.TypeKey]
			if !ok {
				unknown[item.TypeKey]++
				continue
			}
			presenceID, err := profile.NewIdentifier()
			if err != nil {
				return 0, 0, unknown, err
			}
			var identifier any
			if item.Claim == "informed_number" {
				identifier = item.Identifier
			}
			presenceRows = append(presenceRows, []any{
				uuidFromIdentifier(presenceID),
				uuidFromIdentifier(id),
				docType.ID,
				docType.UniquenessPolicy,
				item.Claim,
				identifier,
			})
			seen[item.TypeKey] = true
		}
		if record.Values.CPF != "" && !seen["cpf"] {
			docType, ok := types["cpf"]
			if ok {
				presenceID, err := profile.NewIdentifier()
				if err != nil {
					return 0, 0, unknown, err
				}
				presenceRows = append(presenceRows, []any{
					uuidFromIdentifier(presenceID),
					uuidFromIdentifier(id),
					docType.ID,
					docType.UniquenessPolicy,
					"informed_number",
					record.Values.CPF,
				})
			}
		}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"profiles"}, profileColumns, pgx.CopyFromRows(profileRows)); err != nil {
		return 0, 0, unknown, fmt.Errorf("copy profiles: %w", err)
	}
	if len(presenceRows) > 0 {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"document_presences"}, presenceColumns, pgx.CopyFromRows(presenceRows)); err != nil {
			return 0, 0, unknown, fmt.Errorf("copy presences: %w", err)
		}
	}
	return len(profileRows), len(presenceRows), unknown, nil
}

var profileColumns = []string{
	"id", "full_name", "social_name", "email", "mobile_phone", "landline_phone",
	"address_street", "address_number", "address_complement", "address_neighborhood",
	"address_city", "address_state", "address_postal_code", "notes",
	"birth_date", "gender", "blood_type", "nationality", "birth_city", "marital_status", "wedding_date",
	"father_name", "father_birth_date", "mother_name", "mother_birth_date", "health_plan", "blood_donor", "organ_donor",
	"team", "sector", "collections", "vehicle_model", "vehicle_color", "vehicle_plate", "vehicle_year", "club_membership",
	"membership_type", "place_of_origin", "birth_country", "parents_wedding_date", "supermarket_club", "pet",
	"travel_countries", "card_brand", "card_bank",
}

var presenceColumns = []string{
	"id", "profile_id", "document_type_id", "uniqueness_policy", "claim", "identifier_value",
}

func profileCreateRow(id profile.Identifier, values profile.Values) []any {
	return []any{
		uuidFromIdentifier(id),
		values.FullName,
		nullString(values.SocialName),
		nullString(values.Email),
		nullString(values.MobilePhone),
		nullString(values.LandlinePhone),
		nullString(values.Address.Street),
		nullString(values.Address.Number),
		nullString(values.Address.Complement),
		nullString(values.Address.Neighborhood),
		nullString(values.Address.City),
		nullString(values.Address.State),
		nullString(values.Address.PostalCode),
		nullString(values.Notes),
		dateValue(values.BirthDate),
		nullString(values.Gender),
		nullString(values.BloodType),
		nullString(values.Nationality),
		nullString(values.BirthCity),
		nullString(values.MaritalStatus),
		dateValue(values.WeddingDate),
		nullString(values.FatherName),
		dateValue(values.FatherBirthDate),
		nullString(values.MotherName),
		dateValue(values.MotherBirthDate),
		nullString(values.HealthPlan),
		values.BloodDonor,
		values.OrganDonor,
		nullString(values.Team),
		nullString(values.Sector),
		nullString(values.Collections),
		nullString(values.VehicleModel),
		nullString(values.VehicleColor),
		nullString(values.VehiclePlate),
		values.VehicleYear,
		nullString(values.ClubMembership),
		nullString(values.MembershipType),
		nullString(values.PlaceOfOrigin),
		nullString(values.BirthCountry),
		dateValue(values.ParentsWedding),
		nullString(values.SupermarketClub),
		nullString(values.Pet),
		nullString(values.TravelCountries),
		nullString(values.CardBrand),
		nullString(values.CardBank),
	}
}

func uuidFromIdentifier(id profile.Identifier) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func dateValue(value string) any {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}
	return pgtype.Date{Time: parsed, Valid: true}
}

func MergeCounts(dst, src map[string]int) {
	if src == nil {
		return
	}
	if dst == nil {
		return
	}
	for key, value := range src {
		dst[key] += value
	}
}

func WriteMetadata(ctx context.Context, pool *pgxpool.Pool, report Report) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO app_metadata (key, value) VALUES ('archive.import.dev18', $1)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, string(payload))
	return err
}

func DirectDatabaseURL(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("DATABASE_URL is required")
	}
	return raw, nil
}
