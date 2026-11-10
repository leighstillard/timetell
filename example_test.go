package timetell_test

import (
	"fmt"
	"log"

	"github.com/leighstillard/timetell"
)

func ExampleGraph() {
	cfg := timetell.DefaultConfig()
	cfg.OutputFormat = "full"
	out, err := timetell.Graph("testdata", cfg, "./simple_linear")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(out))
	// Output:
	// flowchart TD
	// s((Start))
	// ProcessData[Process Data]
	// SaveResult[Save Result]
	// ValidateInput[Validate Input]
	// e((End))
	// s --> ValidateInput
	// ValidateInput --> ProcessData
	// ProcessData --> SaveResult
	// SaveResult --> e
	//
	// --- Execution Paths (1 total) ---
	//
	// Path 1: Start → ValidateInput → ProcessData → SaveResult → End
}
