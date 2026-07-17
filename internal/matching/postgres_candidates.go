package matching

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const candidateEvidenceQuery = `WITH exact_pairs AS (
  SELECT left_profile.id AS left_id, right_profile.id AS right_id
  FROM profiles left_profile
  JOIN profiles right_profile ON right_profile.id > left_profile.id AND right_profile.cpf=left_profile.cpf
  WHERE left_profile.cpf IS NOT NULL
  UNION
  SELECT left_profile.id, right_profile.id
  FROM profiles left_profile
  JOIN profiles right_profile ON right_profile.id > left_profile.id AND right_profile.email=left_profile.email
  WHERE left_profile.email IS NOT NULL
  UNION
  SELECT left_profile.id, right_profile.id
  FROM profiles left_profile
  JOIN profiles right_profile ON right_profile.id > left_profile.id AND right_profile.mobile_phone=left_profile.mobile_phone
  WHERE left_profile.mobile_phone IS NOT NULL
  UNION
  SELECT left_profile.id, right_profile.id
  FROM profiles left_profile
  JOIN profiles right_profile ON right_profile.id > left_profile.id AND right_profile.landline_phone=left_profile.landline_phone
  WHERE left_profile.landline_phone IS NOT NULL
), name_pairs AS (
  SELECT left_profile.id AS left_id, candidate.id AS right_id
  FROM profiles left_profile
  CROSS JOIN LATERAL (
    SELECT right_profile.id
    FROM profiles right_profile
    WHERE right_profile.id > left_profile.id
      AND lower(right_profile.full_name) % lower(left_profile.full_name)
    ORDER BY similarity(lower(right_profile.full_name), lower(left_profile.full_name)) DESC, right_profile.id
    LIMIT 25
  ) candidate
), postal_pairs AS (
  SELECT left_profile.id AS left_id, right_profile.id AS right_id
  FROM profiles left_profile
  JOIN profiles right_profile ON right_profile.id > left_profile.id
    AND right_profile.address_postal_code=left_profile.address_postal_code
    AND similarity(lower(right_profile.full_name), lower(left_profile.full_name)) >= 0.45
  WHERE left_profile.address_postal_code IS NOT NULL
), pairs AS (
  SELECT left_id, right_id FROM exact_pairs
  UNION
  SELECT left_id, right_id FROM name_pairs
  UNION
  SELECT left_id, right_id FROM postal_pairs
), evidence AS (
  SELECT pair.left_id, pair.right_id,
         left_profile.version AS left_version, right_profile.version AS right_version,
         item.kind, item.strength, item.contribution
  FROM pairs pair
  JOIN profiles left_profile ON left_profile.id=pair.left_id
  JOIN profiles right_profile ON right_profile.id=pair.right_id
  CROSS JOIN LATERAL (
    SELECT 'CPF_EXACT'::text, 100::integer, 100::integer
    WHERE left_profile.cpf IS NOT NULL AND left_profile.cpf=right_profile.cpf
    UNION ALL
    SELECT 'EMAIL_EXACT', 100, 95
    WHERE left_profile.email IS NOT NULL AND left_profile.email=right_profile.email
    UNION ALL
    SELECT 'MOBILE_EXACT', 100, 90
    WHERE left_profile.mobile_phone IS NOT NULL AND left_profile.mobile_phone=right_profile.mobile_phone
    UNION ALL
    SELECT 'LANDLINE_EXACT', 100, 75
    WHERE left_profile.landline_phone IS NOT NULL AND left_profile.landline_phone=right_profile.landline_phone
    UNION ALL
    SELECT 'NAME_EXACT', 100, 70
    WHERE lower(left_profile.full_name)=lower(right_profile.full_name)
    UNION ALL
    SELECT 'NAME_SIMILAR',
           round(similarity(lower(left_profile.full_name), lower(right_profile.full_name))*100)::integer,
           greatest(35,round(similarity(lower(left_profile.full_name), lower(right_profile.full_name))*65)::integer)
    WHERE lower(left_profile.full_name)<>lower(right_profile.full_name)
      AND similarity(lower(left_profile.full_name), lower(right_profile.full_name)) >= 0.45
    UNION ALL
    SELECT 'POSTAL_EXACT', 100, 20
    WHERE left_profile.address_postal_code IS NOT NULL
      AND left_profile.address_postal_code=right_profile.address_postal_code
    UNION ALL
    SELECT 'CITY_EXACT', 100, 10
    WHERE left_profile.address_city IS NOT NULL
      AND lower(left_profile.address_city)=lower(right_profile.address_city)
    UNION ALL
    SELECT 'ADDRESS_SIMILAR',
           round(similarity(lower(left_profile.address_street), lower(right_profile.address_street))*100)::integer,
           round(similarity(lower(left_profile.address_street), lower(right_profile.address_street))*15)::integer
    WHERE left_profile.address_street IS NOT NULL AND right_profile.address_street IS NOT NULL
      AND similarity(lower(left_profile.address_street), lower(right_profile.address_street)) >= 0.65
  ) item(kind, strength, contribution)
), raw_scores AS (
  SELECT left_id, right_id, left_version, right_version, sum(contribution)::integer AS raw_score
  FROM evidence
  GROUP BY left_id, right_id, left_version, right_version
), limited AS (
  SELECT left_id, right_id, left_version, right_version, least(100,raw_score)::integer AS score
  FROM raw_scores
  WHERE raw_score >= 50
  ORDER BY least(100,raw_score) DESC, left_id, right_id
  LIMIT $1
)
SELECT limited.left_id, limited.right_id, limited.left_version, limited.right_version,
       limited.score, evidence.kind, evidence.strength, evidence.contribution
FROM limited
JOIN evidence USING (left_id, right_id, left_version, right_version)
ORDER BY limited.score DESC, limited.left_id, limited.right_id, evidence.kind`

func (store *PostgresStore) GenerateCandidates(ctx context.Context, analysisID Identifier, maximum int, timeout time.Duration, now time.Time) (AnalysisStats, error) {
	if store == nil || store.pool == nil || analysisID.IsZero() || maximum < 1 || maximum > MaximumCandidates ||
		timeout < time.Second || timeout > time.Minute || now.IsZero() {
		return AnalysisStats{}, ErrInvalidInput
	}
	tx, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return AnalysisStats{}, fmt.Errorf("begin matching candidate generation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var state AnalysisState
	var cancellation pgtype.Timestamptz
	if err := tx.QueryRow(ctx, `SELECT state, cancel_requested_at FROM matching_analyses WHERE id=$1`, matchingUUID(analysisID)).Scan(&state, &cancellation); errors.Is(err, pgx.ErrNoRows) {
		return AnalysisStats{}, ErrNotFound
	} else if err != nil {
		return AnalysisStats{}, fmt.Errorf("load matching analysis state: %w", err)
	}
	if state == AnalysisCancelled || cancellation.Valid {
		return AnalysisStats{}, ErrCancelled
	}
	if state != AnalysisRunning {
		return AnalysisStats{}, ErrInvalidState
	}
	timeoutMilliseconds := timeout.Milliseconds()
	if _, err := tx.Exec(ctx, `SELECT set_config('statement_timeout',$1,true), set_config('pg_trgm.similarity_threshold','0.62',true)`, strconv.FormatInt(timeoutMilliseconds, 10)); err != nil {
		return AnalysisStats{}, fmt.Errorf("configure matching analysis limits: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE matching_cases matching_case
SET state='STALE', version=version+1, updated_at=$1
WHERE state IN ('PENDING','NOT_DUPLICATE')
  AND (
    NOT EXISTS (SELECT 1 FROM profiles profile WHERE profile.id=matching_case.left_profile_id AND profile.version=matching_case.left_profile_version) OR
    NOT EXISTS (SELECT 1 FROM profiles profile WHERE profile.id=matching_case.right_profile_id AND profile.version=matching_case.right_profile_version)
  )`, now); err != nil {
		return AnalysisStats{}, fmt.Errorf("stale changed matching cases: %w", err)
	}
	var stats AnalysisStats
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM profiles`).Scan(&stats.ProfilesScanned); err != nil {
		return AnalysisStats{}, fmt.Errorf("count matching profiles: %w", err)
	}
	candidates, err := readCandidateEvidence(ctx, tx, maximum)
	if err != nil {
		return AnalysisStats{}, normalizePostgresError(err)
	}
	stats.CandidateCount = len(candidates)
	for index, candidate := range candidates {
		if index%50 == 0 {
			var currentState AnalysisState
			if err := tx.QueryRow(ctx, `SELECT state FROM matching_analyses WHERE id=$1`, matchingUUID(analysisID)).Scan(&currentState); err != nil {
				return AnalysisStats{}, fmt.Errorf("check matching cancellation: %w", err)
			}
			if currentState == AnalysisCancelled {
				return AnalysisStats{}, ErrCancelled
			}
		}
		refreshed, err := upsertCandidate(ctx, tx, analysisID, candidate, now)
		if err != nil {
			return AnalysisStats{}, err
		}
		if refreshed {
			stats.RefreshedCount++
		}
	}
	command, err := tx.Exec(ctx, `UPDATE matching_analyses
SET state='COMPLETED', profiles_scanned=$2, candidate_count=$3, refreshed_count=$4,
    completed_at=$5, error_code=NULL, version=version+1, updated_at=$5
WHERE id=$1 AND state='RUNNING' AND cancel_requested_at IS NULL`,
		matchingUUID(analysisID), stats.ProfilesScanned, stats.CandidateCount, stats.RefreshedCount, now)
	if err != nil {
		return AnalysisStats{}, fmt.Errorf("complete matching analysis atomically: %w", err)
	}
	if command.RowsAffected() != 1 {
		return AnalysisStats{}, ErrCancelled
	}
	if err := tx.Commit(ctx); err != nil {
		return AnalysisStats{}, normalizePostgresError(err)
	}
	return stats, nil
}

func readCandidateEvidence(ctx context.Context, tx pgx.Tx, maximum int) ([]Candidate, error) {
	rows, err := tx.Query(ctx, candidateEvidenceQuery, maximum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Candidate, 0)
	var current *Candidate
	for rows.Next() {
		var leftID, rightID pgtype.UUID
		var leftVersion, rightVersion int64
		var score, strength, contribution int
		var kind EvidenceKind
		if err := rows.Scan(&leftID, &rightID, &leftVersion, &rightVersion, &score, &kind, &strength, &contribution); err != nil {
			return nil, fmt.Errorf("scan matching candidate evidence: %w", err)
		}
		left, right := matchingIdentifier(leftID), matchingIdentifier(rightID)
		if current == nil || current.LeftProfileID != left || current.RightProfileID != right {
			result = append(result, Candidate{
				LeftProfileID: left, RightProfileID: right, LeftProfileVersion: leftVersion,
				RightProfileVersion: rightVersion, Score: score, ScoreBand: scoreBand(score),
				Evidence: make([]Evidence, 0, len(evidenceCatalog)),
			})
			current = &result[len(result)-1]
		}
		if !kind.Valid() || strength < 0 || strength > 100 || contribution < 0 || contribution > 100 {
			return nil, ErrInvalidState
		}
		current.Evidence = append(current.Evidence, Evidence{Kind: kind, Strength: strength, Contribution: contribution})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func upsertCandidate(ctx context.Context, tx pgx.Tx, analysisID Identifier, candidate Candidate, now time.Time) (bool, error) {
	if candidate.LeftProfileID.IsZero() || candidate.RightProfileID.IsZero() || candidate.LeftProfileID == candidate.RightProfileID ||
		candidate.LeftProfileVersion <= 0 || candidate.RightProfileVersion <= 0 || candidate.Score < MinimumCandidateScore ||
		candidate.Score > 100 || !candidate.ScoreBand.Valid() || len(candidate.Evidence) == 0 {
		return false, ErrInvalidState
	}
	var caseID pgtype.UUID
	var oldLeftVersion, oldRightVersion int64
	var oldScore int
	var oldState CaseState
	err := tx.QueryRow(ctx, `SELECT id, left_profile_version, right_profile_version, score, state
FROM matching_cases WHERE left_profile_id=$1 AND right_profile_id=$2 FOR UPDATE`,
		matchingUUID(candidate.LeftProfileID), matchingUUID(candidate.RightProfileID)).Scan(&caseID, &oldLeftVersion, &oldRightVersion, &oldScore, &oldState)
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		id, identifierErr := NewIdentifier()
		if identifierErr != nil {
			return false, fmt.Errorf("generate matching case identifier: %w", identifierErr)
		}
		caseID = matchingUUID(id)
		_, err = tx.Exec(ctx, `INSERT INTO matching_cases
(id, left_profile_id, right_profile_id, left_profile_version, right_profile_version,
 score, score_band, state, first_analysis_id, last_analysis_id, created_at, updated_at)
VALUES($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8,$9,$9)`,
			caseID, matchingUUID(candidate.LeftProfileID), matchingUUID(candidate.RightProfileID),
			candidate.LeftProfileVersion, candidate.RightProfileVersion, candidate.Score, candidate.ScoreBand,
			matchingUUID(analysisID), now)
		if err != nil {
			return false, fmt.Errorf("insert matching case: %w", err)
		}
		created = true
	} else if err != nil {
		return false, fmt.Errorf("lock matching case: %w", err)
	}
	if !created && oldState == CaseMerged {
		if _, err := tx.Exec(ctx, `INSERT INTO matching_analysis_cases(analysis_id,case_id,refreshed)
VALUES($1,$2,false) ON CONFLICT (analysis_id,case_id) DO NOTHING`, matchingUUID(analysisID), caseID); err != nil {
			return false, fmt.Errorf("link merged matching case to analysis: %w", err)
		}
		return false, nil
	}
	versionsChanged := !created && (oldLeftVersion != candidate.LeftProfileVersion || oldRightVersion != candidate.RightProfileVersion)
	if !created && oldState == CaseNotDuplicate && !versionsChanged {
		if _, err := tx.Exec(ctx, `INSERT INTO matching_analysis_cases(analysis_id,case_id,refreshed)
VALUES($1,$2,false) ON CONFLICT (analysis_id,case_id) DO NOTHING`, matchingUUID(analysisID), caseID); err != nil {
			return false, fmt.Errorf("link suppressed matching case to analysis: %w", err)
		}
		return false, nil
	}
	refreshed := created || versionsChanged || oldState != CasePending || oldScore != candidate.Score
	if !created {
		_, err = tx.Exec(ctx, `UPDATE matching_cases SET
  left_profile_version=$2, right_profile_version=$3, score=$4, score_band=$5, state='PENDING',
  last_analysis_id=$6, decided_by_user_id=NULL, decided_at=NULL,
  merged_survivor_id=NULL, merged_source_id=NULL, merged_at=NULL,
  version=version+1, updated_at=$7
WHERE id=$1`, caseID, candidate.LeftProfileVersion, candidate.RightProfileVersion,
			candidate.Score, candidate.ScoreBand, matchingUUID(analysisID), now)
		if err != nil {
			return false, fmt.Errorf("refresh matching case: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM matching_case_evidence WHERE case_id=$1`, caseID); err != nil {
		return false, fmt.Errorf("replace matching case evidence: %w", err)
	}
	for _, evidence := range candidate.Evidence {
		if !evidence.Kind.Valid() || evidence.Strength < 0 || evidence.Strength > 100 || evidence.Contribution < 0 || evidence.Contribution > 100 {
			return false, ErrInvalidState
		}
		if _, err := tx.Exec(ctx, `INSERT INTO matching_case_evidence(case_id,evidence_kind,strength,contribution)
VALUES($1,$2,$3,$4)`, caseID, evidence.Kind, evidence.Strength, evidence.Contribution); err != nil {
			return false, fmt.Errorf("insert matching evidence: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO matching_analysis_cases(analysis_id,case_id,refreshed)
VALUES($1,$2,$3) ON CONFLICT (analysis_id,case_id) DO UPDATE SET refreshed=EXCLUDED.refreshed`,
		matchingUUID(analysisID), caseID, refreshed); err != nil {
		return false, fmt.Errorf("link matching case to analysis: %w", err)
	}
	return refreshed, nil
}
