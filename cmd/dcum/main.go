package main

import (
	"fmt"
	"os"

	"github.com/duxet/dcum/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
