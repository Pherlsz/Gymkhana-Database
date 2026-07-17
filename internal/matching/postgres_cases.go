package matching

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Pherlsz/Gymkhana-Database/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const caseDetailColumns = `matching_case.id, matching_case.left_profile_id, matching_case.right_profile_id,
       matching_case.left_profile_version, matching_case.right_profile_version,
       matching_case.score, matching_case.score_band, matching_case.state,
       matching_case.decided_by_user_id, matching_case.decided_at,
       matching_case.merged_survivor_id, matching_case.merged_source_id, matching_case.merged_at,
       matching_case.version, matching_case.created_at, matching_case.updated_at,
       evidence.evidence_kind, evidence.strength, evidence.contribution,
       left_profile.id IS NOT NULL,
       COALESCE(left_profile.full_name,''), COALESCE(left_profile.social_name,''), COALESCE(left_profile.cpf,''),
       COALESCE(left_profile.email,''), COALESCE(left_profile.mobile_phone,''), COALESCE(left_profile.landline_phone,''),
       COALESCE(left_profile.address_street,''), COALESCE(left_profile.address_number,''),
       COALESCE(left_profile.address_complement,''), COALESCE(left_profile.address_neighborhood,''),
       COALESCE(left_profile.address_city,''), COALESCE(left_profile.address_state,''), COALESCE(left_profile.address_postal_code,''),
       COALESCE(left_profile.notes,''), COALESCE(left_profile.version,0), left_profile.updated_at,
       right_profile.id IS NOT NULL,
       COALESCE(right_profile.full_name,''), COALESCE(right_profile.social_name,''), COALESCE(right_profile.cpf,''),
       COALESCE(right_profile.email,''), COALESCE(right_profile.mobile_phone,''), COALESCE(right_profile.landline_phone,''),
       COALESCE(right_profile.address_street,''), COALESCE(right_profile.address_number,''),
       COALESCE(right_profile.address_complement,''), COALESCE(right_profile.address_neighborhood,''),
       COALESCE(right_profile.address_city,''), COALESCE(right_profile.address_state,''), COALESCE(right_profile.address_postal_code,''),
       COALESCE(right_profile.notes,''), COALESCE(right_profile.version,0), right_profile.updated_at`

const caseDetailJoins = `
  LEFT JOIN matching_case_evidence evidence ON evidence.case_id=matching_case.id
  LEFT JOIN profiles left_profile ON left_profile.id=matching_case.left_profile_id
  LEFT JOIN profiles right_profile ON right_profile.id=matching_case.right_profile_id`

func (store *PostgresStore) ListCases(ctx context.Context, options CaseListOptions) (CasePage, error) {
	if store == nil || store.pool == nil {
		return CasePage{}, ErrInvalidInput
	}
	normalized, err := normalizeCaseListOptions(options)
	if err != nil {
		return CasePage{}, err
	}
	states := make([]string, len(normalized.States))
	for index, state := range normalized.States {
		states[index] = string(state)
	}
	bands := make([]string, len(normalized.Bands))
	for index, band := range normalized.Bands {
		bands[index] = string(band)
	}
	var total int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM matching_cases
WHERE state=ANY($1::text[]) AND (cardinality($2::text[])=0 OR score_band=ANY($2::text[]))`, states, bands).Scan(&total); err != nil {
		return CasePage{}, fmt.Errorf("count matching cases: %w", err)
	}
	rows, err := store.pool.Query(ctx, `WITH selected AS (
  SELECT * FROM matching_cases
  WHERE state=ANY($1::text[]) AND (cardinality($2::text[])=0 OR score_band=ANY($2::text[]))
  ORDER BY score DESC, updated_at DESC, id DESC
  LIMIT $3 OFFSET $4
)
SELECT `+caseDetailColumns+`
FROM selected matching_case`+caseDetailJoins+`
ORDER BY matching_case.score DESC, matching_case.updated_at DESC, matching_case.id DESC, evidence.evidence_kind`,
		states, bands, normalized.Limit, normalized.Offset)
	if err != nil {
		return CasePage{}, fmt.Errorf("list matching cases: %w", err)
	}
	values, err := scanCaseRows(rows)
	if err != nil {
		return CasePage{}, err
	}
	return CasePage{Cases: values, Total: total, Limit: normalized.Limit, Offset: normalized.Offset}, nil
}

func (store *PostgresStore) GetCase(ctx context.Context, id Identifier) (Case, error) {
	if store == nil || store.pool == nil || id.IsZero() {
		return Case{}, ErrNotFound
	}
	rows, err := store.pool.Query(ctx, `SELECT `+caseDetailColumns+`
FROM matching_cases matching_case`+caseDetailJoins+`
WHERE matching_case.id=$1
ORDER BY evidence.evidence_kind`, matchingUUID(id))
	if err != nil {
		return Case{}, fmt.Errorf("load matching case: %w", err)
	}
	values, err := scanCaseRows(rows)
	if err != nil {
		return Case{}, err
	}
	if len(values) == 0 {
		return Case{}, ErrNotFound
	}
	return values[0], nil
}

func scanCaseRows(rows pgx.Rows) ([]Case, error) {
	defer rows.Close()
	result := make([]Case, 0)
	var current *Case
	for rows.Next() {
		var value Case
		var id, leftID, rightID, decidedBy, mergedSurvivor, mergedSource pgtype.UUID
		var decidedAt, mergedAt pgtype.Timestamptz
		var evidenceKind pgtype.Text
		var evidenceStrength, evidenceContribution pgtype.Int2
		var leftExists, rightExists bool
		var left, right ProfileSnapshot
		var leftUpdated, rightUpdated pgtype.Timestamptz
		if err := rows.Scan(
			&id, &leftID, &rightID, &value.LeftProfileVersion, &value.RightProfileVersion,
			&value.Score, &value.ScoreBand, &value.State, &decidedBy, &decidedAt,
			&mergedSurvivor, &mergedSource, &mergedAt, &value.Version, &value.CreatedAt, &value.UpdatedAt,
			&evidenceKind, &evidenceStrength, &evidenceContribution,
			&leftExists, &left.FullName, &left.SocialName, &left.CPF, &left.Email, &left.MobilePhone, &left.LandlinePhone,
			&left.AddressStreet, &left.AddressNumber, &left.AddressComplement, &left.AddressNeighborhood,
			&left.AddressCity, &left.AddressState, &left.AddressPostalCode, &left.Notes, &left.Version, &leftUpdated,
			&rightExists, &right.FullName, &right.SocialName, &right.CPF, &right.Email, &right.MobilePhone, &right.LandlinePhone,
			&right.AddressStreet, &right.AddressNumber, &right.AddressComplement, &right.AddressNeighborhood,
			&right.AddressCity, &right.AddressState, &right.AddressPostalCode, &right.Notes, &right.Version, &rightUpdated,
		); err != nil {
			return nil, fmt.Errorf("scan matching case: %w", err)
		}
		value.ID, value.LeftProfileID, value.RightProfileID = matchingIdentifier(id), matchingIdentifier(leftID), matchingIdentifier(rightID)
		if current == nil || current.ID != value.ID {
			if decidedBy.Valid {
				actorID := authIdentifier(decidedBy)
				value.DecidedByUserID = &actorID
			}
			if decidedAt.Valid {
				value.DecidedAt = &decidedAt.Time
			}
			if mergedSurvivor.Valid {
				identifier := matchingIdentifier(mergedSurvivor)
				value.MergedSurvivorID = &identifier
			}
			if mergedSource.Valid {
				identifier := matchingIdentifier(mergedSource)
				value.MergedSourceID = &identifier
			}
			if mergedAt.Valid {
				value.MergedAt = &mergedAt.Time
			}
			if leftExists {
				left.ID = value.LeftProfileID
				if leftUpdated.Valid {
					left.UpdatedAt = leftUpdated.Time
				}
				value.Left = &left
			}
			if rightExists {
				right.ID = value.RightProfileID
				if rightUpdated.Valid {
					right.UpdatedAt = rightUpdated.Time
				}
				value.Right = &right
			}
			value.Evidence = make([]Evidence, 0, len(evidenceCatalog))
			result = append(result, value)
			current = &result[len(result)-1]
		}
		if evidenceKind.Valid {
			kind := EvidenceKind(evidenceKind.String)
			if !kind.Valid() || !evidenceStrength.Valid || !evidenceContribution.Valid {
				return nil, ErrInvalidState
			}
			current.Evidence = append(current.Evidence, Evidence{
				Kind: kind, Strength: int(evidenceStrength.Int16), Contribution: int(evidenceContribution.Int16),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate matching cases: %w", err)
	}
	return result, nil
}

func (store *PostgresStore) DismissCase(ctx context.Context, id Identifier, actorID auth.Identifier, version int64, requestID string, now time.Time) (Case, error) {
	if store == nil || store.pool == nil || id.IsZero() || actorID == (auth.Identifier{}) || version <= 0 ||
		len(requestID) < 1 || len(requestID) > 128 || now.IsZero() {
		return Case{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Case{}, fmt.Errorf("begin matching dismissal: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var state CaseState
	var storedVersion, leftVersion, rightVersion, currentLeftVersion, currentRightVersion int64
	err = tx.QueryRow(ctx, `SELECT matching_case.state, matching_case.version,
       matching_case.left_profile_version, matching_case.right_profile_version,
       left_profile.version, right_profile.version
FROM matching_cases matching_case
JOIN profiles left_profile ON left_profile.id=matching_case.left_profile_id
JOIN profiles right_profile ON right_profile.id=matching_case.right_profile_id
WHERE matching_case.id=$1 FOR UPDATE OF matching_case, left_profile, right_profile`, matchingUUID(id)).Scan(
		&state, &storedVersion, &leftVersion, &rightVersion, &currentLeftVersion, &currentRightVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Case{}, ErrNotFound
	}
	if err != nil {
		return Case{}, fmt.Errorf("lock matching case for dismissal: %w", err)
	}
	if state != CasePending {
		return Case{}, ErrInvalidState
	}
	if storedVersion != version || leftVersion != currentLeftVersion || rightVersion != currentRightVersion {
		return Case{}, ErrConflict
	}
	command, err := tx.Exec(ctx, `UPDATE matching_cases
SET state='NOT_DUPLICATE', decided_by_user_id=$2, decided_at=$3, version=version+1, updated_at=$3
WHERE id=$1 AND version=$4`, matchingUUID(id), matchingAuthUUID(actorID), now, version)
	if err != nil {
		return Case{}, fmt.Errorf("dismiss matching case: %w", err)
	}
	if command.RowsAffected() != 1 {
		return Case{}, ErrConflict
	}
	decisionID, err := NewIdentifier()
	if err != nil {
		return Case{}, fmt.Errorf("generate matching decision identifier: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO matching_case_decisions
(id,case_id,actor_user_id,action,left_profile_version,right_profile_version,request_id,decided_at)
VALUES($1,$2,$3,'NOT_DUPLICATE',$4,$5,$6,$7)`,
		matchingUUID(decisionID), matchingUUID(id), matchingAuthUUID(actorID), leftVersion, rightVersion, requestID, now); err != nil {
		return Case{}, fmt.Errorf("record matching dismissal decision: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, normalizePostgresError(err)
	}
	return store.GetCase(ctx, id)
}
