// Package golden compares test output with checked-in expected files.
package golden

import (
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files with current output")

// Check fails t unless got equals the contents of path. With -update it
// rewrites path instead.
func Check(t testing.TB, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test -update to create it)", err)
	}
	if string(got) != string(want) {
		t.Errorf("%s mismatch (go test -update rewrites it)\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
