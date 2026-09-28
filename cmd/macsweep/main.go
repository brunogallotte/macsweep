package main

import (
	"fmt"
	"os"

	"github.com/brunogallotte/macsweep/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "macsweep:", err)
		os.Exit(1)
	}
}
