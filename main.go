package main

import (
	"os"

	"github.com/debabky/relayer/internal/cli"
)

func main() {
	if !cli.Run(os.Args) {
		os.Exit(1)
	}
}
