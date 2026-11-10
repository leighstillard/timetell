package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "timetell")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	td, _ := filepath.Abs("../../testdata")
	tool := func(args ...string) (string, string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = td
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return stdout.String(), stderr.String(), code
	}

	mmd, _ := os.ReadFile(filepath.Join(td, "money_transfer/expected.mmd"))
	if out, errs, code := tool("graph", "./money_transfer"); code != 0 || out != string(mmd) {
		t.Errorf("exit %d, stderr %q, stdout:\n%s", code, errs, out)
	}
	capped, _ := os.ReadFile(filepath.Join(td, "money_transfer/expected_paths_capped.txt"))
	if out, _, _ := tool("graph", "-OutputFormat", "paths", "-MaxPaths", "2", "./money_transfer"); out != string(capped) {
		t.Errorf("flags not applied:\n%s", out)
	}
	if out, _, _ := tool("graph", "-workflow", "PaymentWorkflow", "./parent_child"); strings.Contains(out, "%%") || !strings.Contains(out, "ProcessPayment") {
		t.Errorf("-workflow:\n%s", out)
	}
	o := filepath.Join(t.TempDir(), "g.mmd")
	if out, _, code := tool("graph", "-o", o, "./money_transfer"); code != 0 || out != "" {
		t.Errorf("-o: exit %d, stdout %q", code, out)
	}
	if got, _ := os.ReadFile(o); !bytes.Equal(got, mmd) {
		t.Errorf("-o wrote %q", got)
	}
	for _, args := range [][]string{
		{"graph", "./no_such_dir"},
		{"graph", "-workflow", "Nope", "./money_transfer"},
		{"graph", "-Direction", "BT", "./money_transfer"},
		{"graph", "-bogus"},
		{"lint"},
	} {
		if _, errs, code := tool(args...); code == 0 || errs == "" {
			t.Errorf("%v: exit %d, stderr %q", args, code, errs)
		}
	}
	if _, errs, code := tool("graph", "-h"); code != 0 || !strings.Contains(errs, "-OutputFormat") {
		t.Errorf("help lists no flags:\n%s", errs)
	}
}
