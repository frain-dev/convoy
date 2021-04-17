package main

import (
	"log"
	"os"

	cli "github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:  "hookstack",
		Usage: "Easy webhook management",
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
