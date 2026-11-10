// Package timetell draws static Mermaid graphs of Temporal Go workflows.
package timetell

import (
	"fmt"
	"go/token"
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
	Direction         string // TD (default) or LR
	OutputFormat      string // mermaid, paths or full
}

// DefaultConfig is Python's defaults, except Direction TD.
func DefaultConfig() Config {
	return Config{
		StartLabel: "Start", EndLabel: "End",
		DecisionTrueLabel: "yes", DecisionFalseLabel: "no",
		SignalSuccessLabel: "Signaled", SignalTimeoutLabel: "Timeout",
		SplitNamesByWords: true, MaxDecisionPoints: 10,
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
	case cfg.OutputFormat == "paths":
		return nil, fmt.Errorf("OutputFormat paths is not implemented yet (timetell-pas.8)")
	case cfg.OutputFormat != "mermaid" && cfg.OutputFormat != "full":
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
	var b strings.Builder
	for _, r := range roots {
		g := flow.Build(r, words)
		if len(roots) > 1 {
			b.WriteString("%% " + g.Workflow + "\n")
		}
		b.Write(render.Mermaid(g, opts))
		if cfg.OutputFormat == "full" {
			b.WriteString(report(g, r, cfg.MaxDecisionPoints, abs))
		}
	}
	return []byte(b.String()), nil
}

// report is the validation report for g, empty when there are no warnings.
func report(g *flow.Graph, r *discover.Func, max int, dir string) string {
	ws := g.Warnings
	n := 0
	for _, nd := range g.Nodes {
		if nd.Kind == flow.Decision {
			n++
		}
	}
	// ponytail: counts decisions only; signal waits join the count with timetell-pas.9.
	if n > max {
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
