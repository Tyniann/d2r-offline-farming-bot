package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
)

func main() {
	source := flag.String("source", "", "absolute source configs directory")
	output := flag.String("output", "", "absolute default bundle output directory")
	flag.Parse()
	if !filepath.IsAbs(*source) || !filepath.IsAbs(*output) {
		fmt.Fprintln(os.Stderr, "source and output must be absolute")
		os.Exit(2)
	}
	if err := app.BuildDefaultBundle(*source, *output); err != nil {
		fmt.Fprintf(os.Stderr, "build default bundle: %v\n", err)
		os.Exit(1)
	}
}
