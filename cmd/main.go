package main

import (
	"github.com/frain-dev/convoy/cmd/cli"
)

func main() {
	slog, c := cli.Build()

	if err := c.Execute(); err != nil {
		slog.Fatal(err)
	}
}
