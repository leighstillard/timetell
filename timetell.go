// Package timetell draws static Mermaid graphs of Temporal Go workflows.
package timetell

import (
	"fmt"
	"go/token"
	"math/big"
	"path/filepath"
	"slices"
	"strings"

	"github.com/leighstillard/timetell/internal/discover"
	"github.com/leighstillard/timetell/internal/flow"
	"github.com/leighstillard/timetell/internal/load"
	"github.com/leighstillard/timetell/internal/render"
)

// Config mirrors Python's GraphBuildingContext; dropped and fixed fields are
// listed in docs/analysis.md (Config bullet).
type Config struct {
	StartLabel, EndLabel                  string
	DecisionTrueLabel, DecisionFalseLabel string
	// SignalSuccessLabel and SignalTimeoutLabel label signal waits (timetell-pas.9).
	SignalSuccessLabel, SignalTimeoutLabel string
	// SplitNamesByWords splits camelCase activity (and, later, signal) names.
	SplitNamesByWords bool
	// MaxDecisionPoints: more decisions than this in one workflow is a warning.
	MaxDecisionPoints int
	MaxPaths          int    // caps the listed paths, not the count
	Direction         string // TD (default) or LR
	OutputFormat      string // mermaid, paths or full
	Workflow          string // graph only the root of this name; "" = all
}

// DefaultConfig is Python's defaults, except Direction TD.
func DefaultConfig() Config {
	return Config{
		StartLabel: "Start", EndLabel: "End",
		DecisionTrueLabel: "yes", DecisionFalseLabel: "no",
		SignalSuccessLabel: "Signaled", SignalTimeoutLabel: "Timeout",
		SplitNamesByWords: true, MaxDecisionPoints: 10, MaxPaths: 1024,
		Direction: "TD", OutputFormat: "mermaid",
	}
}

// Graph loads patterns relative to dir and renders every workflow root in
// cfg.OutputFormat. Several roots are drawn one after another, sorted by
// name, each headed by a %% Name comment. Warning positions are relative to dir.
func Graph(dir string, cfg Config, patterns ...string) ([]byte, error) {
	switch {
	case cfg.Direction != "TD" && cfg.Direction != "LR":
		return nil, fmt.Errorf("Direction %q: want TD or LR", cfg.Direction)
	case slices.Contains([]string{cfg.StartLabel, cfg.EndLabel, cfg.DecisionTrueLabel, cfg.DecisionFalseLabel,
		cfg.SignalSuccessLabel, cfg.SignalTimeoutLabel}, ""):
		return nil, fmt.Errorf("labels must not be empty (Mermaid cannot parse an empty label)")
	case cfg.MaxPaths < 0:
		return nil, fmt.Errorf("MaxPaths %d: want 0 or more", cfg.MaxPaths)
	case cfg.OutputFormat != "mermaid" && cfg.OutputFormat != "paths" && cfg.OutputFormat != "full":
		return nil, fmt.Errorf("OutputFormat %q: want mermaid, paths or full", cfg.OutputFormat)
	}
	pkgs, err := load.Load(dir, patterns...)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err == nil { // go/packages reports real paths; dir may be under a symlink
		abs, err = filepath.EvalSymlinks(abs)
	}
	if err != nil {
		return nil, err
	}
	words := flow.Labels{Start: cfg.StartLabel, End: cfg.EndLabel, Yes: cfg.DecisionTrueLabel, No: cfg.DecisionFalseLabel}
	opts := render.Options{Direction: cfg.Direction, SplitNamesByWords: cfg.SplitNamesByWords}
	roots := discover.Discover(pkgs).Roots
	if cfg.Workflow != "" {
		roots = slices.DeleteFunc(roots, func(r *discover.Func) bool { return r.Obj.Name() != cfg.Workflow })
		if len(roots) == 0 {
			return nil, fmt.Errorf("no root workflow named %q", cfg.Workflow)
		}
	}
	var b strings.Builder
	for _, r := range roots {
		g := flow.Build(r, words)
		if len(roots) > 1 {
			b.WriteString("%% " + g.Workflow + "\n")
		}
		if cfg.OutputFormat != "paths" {
			b.Write(render.Mermaid(g, opts))
		}
		if cfg.OutputFormat != "mermaid" {
			b.WriteString(paths(g, cfg.MaxPaths))
		}
		if cfg.OutputFormat == "full" {
			b.WriteString(report(g, r, cfg.MaxDecisionPoints, abs))
		}
	}
	return []byte(b.String()), nil
}

// ponytail: counts decisions only; signal waits join the count with timetell-pas.9.
func decisions(g *flow.Graph) int {
	n := 0
	for _, nd := range g.Nodes {
		if nd.Kind == flow.Decision {
			n++
		}
	}
	return n
}

// paths is Python's path list: Start, the activities in order, End. Python's
// "(2^n = N paths)" is dropped: the count here follows the structure, not 2^n.
func paths(g *flow.Graph, max int) string {
	total, list := g.Paths(max)
	var b strings.Builder
	fmt.Fprintf(&b, "\n--- Execution Paths (%s total) ---\n", total)
	if n := decisions(g); n > 0 {
		fmt.Fprintf(&b, "Decision Points: %d\n", n)
	}
	if total.Cmp(big.NewInt(int64(len(list)))) > 0 {
		fmt.Fprintf(&b, "Listing the first %d (MaxPaths)\n", len(list))
	}
	b.WriteString("\n")
	for i, p := range list {
		var steps []string
		for _, n := range p {
			switch n.Kind {
			case flow.Start, flow.End, flow.Activity, flow.LocalActivity, flow.Unknown:
				steps = append(steps, n.Label)
			}
		}
		fmt.Fprintf(&b, "Path %d: %s\n", i+1, strings.Join(steps, " → "))
	}
	return b.String()
}

// report is the validation report for g, empty when there are no warnings.
func report(g *flow.Graph, r *discover.Func, max int, dir string) string {
	ws := g.Warnings
	if n := decisions(g); n > max {
		ws = append(ws, load.Warning{Pos: r.Pkg.Fset.Position(r.Decl.Pos()),
			Msg: fmt.Sprintf("%d decision points, more than MaxDecisionPoints (%d)", n, max)})
	}
	if len(ws) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n--- Validation Report ---\nWarnings: %d\n\n", len(ws))
	for _, w := range ws {
		w.Pos = rel(w.Pos, dir)
		b.WriteString(w.String() + "\n")
	}
	return b.String()
}

func rel(p token.Position, dir string) token.Position {
	if r, err := filepath.Rel(dir, p.Filename); err == nil {
		p.Filename = filepath.ToSlash(r)
	}
	return p
}
