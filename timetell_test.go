package timetell

import (
	"strings"
	"testing"

	"github.com/leighstillard/timetell/internal/golden"
)

func TestGraphGolden(t *testing.T) {
	custom := DefaultConfig()
	custom.StartLabel, custom.EndLabel = "BEGIN", "FINISH"
	custom.DecisionTrueLabel, custom.DecisionFalseLabel = "Y", "N"
	custom.Direction = "LR"
	nosplit := DefaultConfig()
	nosplit.SplitNamesByWords = false
	full := DefaultConfig()
	full.OutputFormat = "full"
	for _, c := range []struct {
		fixture, golden string
		cfg             Config
	}{
		{"money_transfer", "expected.mmd", DefaultConfig()}, // defaults match the renderer's goldens
		{"money_transfer", "expected_labels.mmd", custom},
		{"money_transfer", "expected_nosplit.mmd", nosplit},
		{"money_transfer", "expected.mmd", full}, // no warnings: no report
		{"classify", "expected_full.txt", full},
	} {
		t.Run(c.fixture+"/"+c.golden, func(t *testing.T) {
			out, err := Graph("testdata", c.cfg, "./"+c.fixture)
			if err != nil {
				t.Fatal(err)
			}
			golden.Check(t, "testdata/"+c.fixture+"/"+c.golden, out)
		})
	}
}

func TestMaxDecisionPoints(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OutputFormat, cfg.MaxDecisionPoints = "full", 1
	out, err := Graph("testdata", cfg, "./money_transfer")
	if err != nil {
		t.Fatal(err)
	}
	want := "\n\nmoney_transfer/workflow.go:20:1: 2 decision points, more than MaxDecisionPoints (1)\n"
	if !strings.HasSuffix(string(out), want) {
		t.Errorf("missing %q:\n%s", want, out)
	}
}

func TestConfigErrors(t *testing.T) {
	for _, mut := range []func(*Config){
		func(c *Config) { c.Direction = "BT" },
		func(c *Config) { c.StartLabel = "" },
		func(c *Config) { c.DecisionFalseLabel = "" },
		func(c *Config) { c.OutputFormat = "json" },
		func(c *Config) { c.OutputFormat = "paths" },
	} {
		cfg := DefaultConfig()
		mut(&cfg)
		if _, err := Graph("testdata", cfg, "./trivial"); err == nil {
			t.Errorf("%+v: no error", cfg)
		}
	}
}

// Several roots: one graph each, headed by %% Name.
func TestSeveralRoots(t *testing.T) {
	out, err := Graph("testdata", DefaultConfig(), "./parent_child")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(out), "%% "); n < 2 {
		t.Errorf("%d headers:\n%s", n, out)
	}
}
