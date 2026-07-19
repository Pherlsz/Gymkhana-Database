package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Pherlsz/Gymkhana-Database/internal/legacymigration"
)

func main() {
	bundle := flag.String("bundle", "", "directory containing a legacy migration bundle")
	reportPath := flag.String("report", "", "optional path for the JSON reconciliation report")
	strict := flag.Bool("strict", false, "treat warnings as a failed rehearsal")
	flag.Parse()
	if *bundle == "" {
		fmt.Fprintln(os.Stderr, "--bundle is required")
		os.Exit(2)
	}
	report, err := legacymigration.ValidateBundle(*bundle)
	if err != nil {
		fmt.Fprintln(os.Stderr, "legacy bundle validation failed:", err)
		os.Exit(1)
	}
	if *reportPath != "" {
		if err := legacymigration.WriteReport(*reportPath, report); err != nil {
			fmt.Fprintln(os.Stderr, "write reconciliation report:", err)
			os.Exit(1)
		}
	}
	encoded, _ := json.Marshal(report)
	fmt.Println(string(encoded))
	if !report.Valid || (*strict && len(report.Warnings) > 0) {
		os.Exit(1)
	}
}
