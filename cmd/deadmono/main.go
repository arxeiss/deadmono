// Package main provides CLI for running deadcode analysis across monorepo.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/arxeiss/deadmono/analysis"
)

var (
	debugFlag = flag.Bool("debug", false, "enable debug output")
	helpFlag  = flag.Bool("help", false, "show help")

	testFlag = flag.Bool("test", false, "include implicit test packages and executables (deadcode flag)")
	tagsFlag = flag.String("tags", "",
		"comma-separated list of extra build tags (see: go help buildconstraint) (deadcode flag)")

	generatedFlag = flag.Bool("generated", false, "include dead functions in generated Go files (deadcode flag)")
	jsonFlag      = flag.Bool("json", false, "output JSON records (deadcode flag)")
)

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 || *helpFlag {
		usage()
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	runner := analysis.New(os.Stdout, os.Stderr, flag.Args())
	runner.DebugFlag = *debugFlag
	runner.GeneratedFlag = *generatedFlag
	runner.TestFlag = *testFlag
	runner.TagsFlag = *tagsFlag
	runner.JSONFlag = *jsonFlag

	err := runner.Run(ctx)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())

		exitCode := 1
		cancel()
		os.Exit(exitCode)
	}
}

func usage() {
	// Extract the content of the /* ... */ comment in doc.go.
	_, _ = os.Stderr.WriteString(`DOC HERE
Flags:

`)
	flag.PrintDefaults()
}
