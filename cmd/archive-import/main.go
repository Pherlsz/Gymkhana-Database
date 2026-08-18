package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Pherlsz/Gymkhana-Database/internal/archiveimport"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

func main() {
	file := flag.String("file", "", "path to arquivo-unico-cadastro.xlsx")
	sheet := flag.String("sheet", "cadastro", "workbook sheet")
	batch := flag.Int("batch", 250, "profiles per transaction")
	dryRun := flag.Bool("dry-run", false, "map and normalize without writing")
	replace := flag.Bool("replace", false, "delete existing Dev-18 archive rows then load")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--file is required")
		os.Exit(2)
	}
	databaseURL := strings.Replace(strings.TrimSpace(os.Getenv("DATABASE_URL")), "-pooler.", ".", 1)
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workbook, err := excelize.OpenFile(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open workbook:", err)
		os.Exit(1)
	}
	defer workbook.Close()

	rows, err := workbook.Rows(*sheet)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open sheet:", err)
		os.Exit(1)
	}
	defer rows.Close()

	if !rows.Next() {
		fmt.Fprintln(os.Stderr, "workbook has no header row")
		os.Exit(1)
	}
	header, err := rows.Columns()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read header:", err)
		os.Exit(1)
	}

	report := archiveimport.Report{
		SkipReasons:  map[string]int{},
		Dropped:      map[string]int{},
		UnknownTypes: map[string]int{},
	}

	var pool *pgxpool.Pool
	var types map[string]archiveimport.DocumentType
	if !*dryRun {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err != nil {
			fmt.Fprintln(os.Stderr, "connect:", err)
			os.Exit(1)
		}
		defer pool.Close()
		if err := archiveimport.GuardDevProject(ctx, pool, *replace); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		types, err = archiveimport.LoadTypes(ctx, pool)
		if err != nil {
			fmt.Fprintln(os.Stderr, "load document types:", err)
			os.Exit(1)
		}
	}

	pending := make([]archiveimport.Record, 0, *batch)
	flush := func() error {
		if *dryRun || len(pending) == 0 {
			pending = pending[:0]
			return nil
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		inserted, presences, unknown, err := archiveimport.InsertBatch(ctx, tx, types, pending)
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		report.Inserted += inserted
		report.Presences += presences
		archiveimport.MergeCounts(report.UnknownTypes, unknown)
		pending = pending[:0]
		fmt.Fprintf(os.Stderr, "inserted %d profiles\n", report.Inserted)
		return nil
	}

	for rows.Next() {
		cells, err := rows.Columns()
		if err != nil {
			fmt.Fprintln(os.Stderr, "read row:", err)
			os.Exit(1)
		}
		report.Read++
		record := archiveimport.MapRow(header, cells)
		record, dropped := archiveimport.NormalizeRecord(record)
		for _, item := range dropped {
			report.Dropped[item]++
		}
		if record.Skip || record.Values.FullName == "" {
			reason := record.SkipReason
			if reason == "" {
				reason = "missing_name"
			}
			report.Skipped++
			report.SkipReasons[reason]++
			continue
		}
		if *dryRun {
			report.Inserted++
			report.Presences += len(record.Presences)
			continue
		}
		pending = append(pending, record)
		if len(pending) >= *batch {
			if err := flush(); err != nil {
				fmt.Fprintln(os.Stderr, "insert batch:", err)
				os.Exit(1)
			}
		}
	}
	if err := rows.Error(); err != nil {
		fmt.Fprintln(os.Stderr, "iterate rows:", err)
		os.Exit(1)
	}
	if err := flush(); err != nil {
		fmt.Fprintln(os.Stderr, "insert batch:", err)
		os.Exit(1)
	}
	if !*dryRun {
		if err := archiveimport.WriteMetadata(ctx, pool, report); err != nil {
			fmt.Fprintln(os.Stderr, "write metadata:", err)
			os.Exit(1)
		}
	}
	encoded, _ := json.Marshal(report)
	fmt.Println(string(encoded))
}
