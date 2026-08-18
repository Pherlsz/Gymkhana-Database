package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pherlsz/Gymkhana-Database/internal/config"
	"github.com/Pherlsz/Gymkhana-Database/internal/document"
	"github.com/Pherlsz/Gymkhana-Database/internal/platform/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type cleanupReport struct {
	Scanned int            `json:"scanned"`
	Updated int            `json:"updated"`
	Deleted int            `json:"deleted"`
	Kept    int            `json:"kept"`
	States  int            `json:"states"`
	DryRun  bool           `json:"dry_run"`
	Samples []cleanupSample `json:"samples,omitempty"`
}

type cleanupSample struct {
	ID     string `json:"id"`
	Before string `json:"before"`
	Action string `json:"action"`
	Number string `json:"number,omitempty"`
	State  string `json:"state,omitempty"`
}

func main() {
	dryRun := flag.Bool("dry-run", true, "print actions without writing")
	apply := flag.Bool("apply", false, "write classified identifiers to the database")
	flag.Parse()
	if *apply {
		*dryRun = false
	}
	if err := run(*dryRun); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dryRun bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	if pool == nil {
		return fmt.Errorf("database connection was not opened")
	}
	defer pool.Close()
	report, err := cleanupIdentifiers(ctx, pool, dryRun)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func cleanupIdentifiers(ctx context.Context, pool *pgxpool.Pool, dryRun bool) (cleanupReport, error) {
	rows, err := pool.Query(ctx, `SELECT document.id::text, COALESCE(presence.identifier_value, ''),
  COALESCE((
    SELECT value.text_value
    FROM custom_field_values value
    JOIN custom_field_definitions field ON field.id = value.field_definition_id
    WHERE value.document_id = document.id AND field.technical_key IN ('state', 'uf', 'issuing_state')
    LIMIT 1
  ), '')
FROM documents document
JOIN document_presences presence ON presence.id = document.presence_id`)
	if err != nil {
		return cleanupReport{}, err
	}
	defer rows.Close()
	report := cleanupReport{DryRun: dryRun}
	type pending struct {
		id, before, number, state string
		action                    document.IdentifierAction
	}
	var work []pending
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.before, &item.state); err != nil {
			return cleanupReport{}, err
		}
		report.Scanned++
		classified := document.ClassifyIdentifier(item.before)
		item.action = classified.Action
		item.number = classified.Number
		if classified.State != "" && item.state == "" {
			item.state = classified.State
		} else if classified.State == "" {
			item.state = ""
		}
		if classified.Action == document.IdentifierDelete {
			work = append(work, item)
			continue
		}
		if classified.Number != item.before || (classified.State != "" && item.state == classified.State) {
			work = append(work, item)
		}
	}
	if err := rows.Err(); err != nil {
		return cleanupReport{}, err
	}
	for _, item := range work {
		if len(report.Samples) < 40 {
			report.Samples = append(report.Samples, cleanupSample{
				ID: item.id, Before: item.before, Action: string(item.action), Number: item.number, State: item.state,
			})
		}
		if item.action == document.IdentifierDelete {
			report.Deleted++
			if dryRun {
				continue
			}
			if _, err := pool.Exec(ctx, `UPDATE document_presences AS presence
SET claim = 'indication', identifier_value = NULL, version = presence.version + 1, updated_at = now()
FROM documents AS document
WHERE document.id = $1::uuid AND presence.id = document.presence_id`, item.id); err != nil {
				return report, err
			}
			if _, err := pool.Exec(ctx, `DELETE FROM document_current_uses WHERE document_id=$1`, item.id); err != nil {
				return report, err
			}
			if _, err := pool.Exec(ctx, `DELETE FROM custom_field_values WHERE document_id=$1`, item.id); err != nil {
				return report, err
			}
			if _, err := pool.Exec(ctx, `DELETE FROM documents WHERE id=$1`, item.id); err != nil {
				return report, err
			}
			continue
		}
		if item.number != item.before {
			report.Updated++
			if !dryRun {
				var identifier any
				if item.number == "" {
					identifier = nil
				} else {
					identifier = item.number
				}
				if _, err := pool.Exec(ctx, `UPDATE document_presences AS presence
SET claim = CASE WHEN $2::text IS NULL THEN 'indication' ELSE 'informed_number' END,
  identifier_value = $2, version = presence.version + 1, updated_at = now()
FROM documents AS document
WHERE document.id = $1::uuid AND presence.id = document.presence_id`, item.id, identifier); err != nil {
					return report, err
				}
			}
		} else {
			report.Kept++
		}
		if item.state != "" {
			report.States++
			if !dryRun {
				if err := upsertDocumentState(ctx, pool, item.id, item.state); err != nil {
					return report, err
				}
			}
		}
	}
	return report, nil
}

func upsertDocumentState(ctx context.Context, pool *pgxpool.Pool, documentID, state string) error {
	_, err := pool.Exec(ctx, `
INSERT INTO custom_field_values (id, field_definition_id, document_id, field_kind, text_value)
SELECT gen_random_uuid(), field.id, document.id, field.field_kind, $2
FROM documents document
JOIN document_presences presence ON presence.id = document.presence_id
JOIN custom_field_definitions field ON field.document_type_id = presence.document_type_id
  AND field.technical_key IN ('state', 'uf', 'issuing_state')
  AND field.field_kind IN ('TEXT', 'LONG_TEXT')
WHERE document.id = $1::uuid
  AND NOT EXISTS (
    SELECT 1 FROM custom_field_values existing
    WHERE existing.document_id = document.id AND existing.field_definition_id = field.id
      AND COALESCE(existing.text_value, '') <> ''
  )`, documentID, state)
	return err
}
