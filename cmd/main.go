package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/arxeiss/deadmono/analysis"
)

var (
	debugFlag = flag.Bool("debug", false, "enable debug output")
	helpFlag  = flag.Bool("help", false, "show help")

	testFlag = flag.Bool("test", false, "include implicit test packages and executables (deadcode flag)")
	tagsFlag = flag.String("tags", "", "comma-separated list of extra build tags (see: go help buildconstraint) (deadcode flag)")

	generatedFlag = flag.Bool("generated", false, "include dead functions in generated Go files (deadcode flag)")
	// filterFlag    = flag.String("filter", "<module>", "report only packages matching this regular expression (default: module of first package) (deadcode flag)")
	jsonFlag = flag.Bool("json", false, "output JSON records (deadcode flag)")
)

// TODO:
// - flags for help
func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if len(flag.Args()) == 0 || *helpFlag {
		usage()
		os.Exit(2)
	}

	runner := analysis.New(os.Stdout, os.Stderr, flag.Args())
	runner.DebugFlag = *debugFlag
	runner.GeneratedFlag = *generatedFlag
	runner.TestFlag = *testFlag
	runner.TagsFlag = *tagsFlag
	runner.JSONFlag = *jsonFlag

	err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())

		exitCode := 1
		os.Exit(exitCode)
	}
}

func usage() {
	// Extract the content of the /* ... */ comment in doc.go.
	io.WriteString(os.Stderr, `DOC HERE
Flags:

`)
	flag.PrintDefaults()
}
