// Command timetell draws static Mermaid graphs of Temporal Go workflows.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "usage: timetell graph|lint|diff [flags] [packages]")
	os.Exit(2)
}
