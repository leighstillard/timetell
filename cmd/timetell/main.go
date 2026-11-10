// Command timetell draws static Mermaid graphs of Temporal Go workflows.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/leighstillard/timetell"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	stderr := os.Stderr
	if len(args) == 0 || args[0] != "graph" {
		fmt.Fprintln(stderr, "usage: timetell graph [flags] [packages]")
		return 2
	}
	cfg := timetell.DefaultConfig()
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: timetell graph [flags] [packages] (default ./...)\n\nflags:")
		fs.PrintDefaults()
	}
	// Flags carry the Config field names (docs/analysis.md, "Port as-is").
	fs.StringVar(&cfg.StartLabel, "StartLabel", cfg.StartLabel, "label of the start node")
	fs.StringVar(&cfg.EndLabel, "EndLabel", cfg.EndLabel, "label of the end node")
	fs.StringVar(&cfg.DecisionTrueLabel, "DecisionTrueLabel", cfg.DecisionTrueLabel, "label of a decision's true edge")
	fs.StringVar(&cfg.DecisionFalseLabel, "DecisionFalseLabel", cfg.DecisionFalseLabel, "label of a decision's false edge")
	fs.StringVar(&cfg.SignalSuccessLabel, "SignalSuccessLabel", cfg.SignalSuccessLabel, "label of a signal wait's success edge")
	fs.StringVar(&cfg.SignalTimeoutLabel, "SignalTimeoutLabel", cfg.SignalTimeoutLabel, "label of a signal wait's timeout edge")
	fs.BoolVar(&cfg.SplitNamesByWords, "SplitNamesByWords", cfg.SplitNamesByWords, "split camelCase activity names into words")
	fs.IntVar(&cfg.MaxDecisionPoints, "MaxDecisionPoints", cfg.MaxDecisionPoints, "warn above this many decisions in one workflow")
	fs.IntVar(&cfg.MaxPaths, "MaxPaths", cfg.MaxPaths, "list at most this many paths")
	fs.StringVar(&cfg.Direction, "Direction", cfg.Direction, "flowchart direction: TD or LR")
	fs.StringVar(&cfg.OutputFormat, "OutputFormat", cfg.OutputFormat, "mermaid, paths or full")
	fs.StringVar(&cfg.Workflow, "workflow", "", "graph only the root workflow with this name")
	out := fs.String("o", "", "write to this file instead of stdout")
	if err := fs.Parse(args[1:]); err == flag.ErrHelp {
		return 0
	} else if err != nil {
		return 2
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	b, err := timetell.Graph(".", cfg, patterns...)
	if err != nil {
		fmt.Fprintln(stderr, "timetell:", err)
		return 1
	}
	if *out != "" {
		err = os.WriteFile(*out, b, 0o644)
	} else {
		_, err = os.Stdout.Write(b)
	}
	if err != nil {
		fmt.Fprintln(stderr, "timetell:", err)
		return 1
	}
	return 0
}
