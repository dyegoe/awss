package main

import (
	"os"

	"github.com/dyegoe/awss/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
