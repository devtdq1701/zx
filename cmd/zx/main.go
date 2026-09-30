package main

import (
	"fmt"
	"os"
	_ "time/tzdata"

	"zx/internal/cli"
)

func main() {
	if len(os.Args) == 1 {
		if err := cli.RunREPL(); err != nil {
			fmt.Fprintf(os.Stderr, "REPL error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
