/*
The deadmono command reports unreachable functions across multiple entrypoints in Go monorepos.

	Usage: deadmono [flags] path/to/main1.go path/to/main2.go ...

The deadmono command extends the functionality of the deadcode tool
(https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) to work with monorepos
containing multiple main packages. It analyzes each entrypoint separately,
then reports only the functions that are unreachable from ALL entrypoints.

This is particularly useful in monorepo setups where you have shared packages
used by multiple services. A function might be dead code from one service's
perspective but actively used by another service. deadmono helps identify
truly unused code by finding the package based intersection of dead code across all services.

# How it works

For each provided entrypoint (main.go file):
 1. Lists all dependencies
 2. Runs deadcode analysis to find unreachable functions
 3. Intersects package based results across all entrypoints
 4. Reports only functions that are dead in ALL entrypoints

# Example

Analyze three services in a monorepo:

	$ deadmono services/authn/main.go services/config/main.go services/healthcheck/main.go

This will report functions that are unused by all three services.

# Flags

The -test flag causes it to analyze test executables too (passed to deadcode).

The -generated flag includes dead functions in generated Go files (passed to deadcode).

The -tags flag allows specifying build tags (passed to deadcode).

The -filter flag allows filtering packages by regular expression (passed to deadcode).
By default, it filters to the module of the first entrypoint ("<module>").
When using a custom filter, entrypoints from different Go modules are supported.

The -json flag outputs results in JSON format (same format as deadcode).

The -debug flag enables verbose debug output.

# Output

The output format matches deadcode, with one difference: file path handling.
Since deadmono analyzes multiple entrypoints, it uses a consistent path strategy:

  - Single module: Paths relative to go.mod when all entrypoints are in the same module
  - Multiple modules: Absolute paths when entrypoints span different modules

# Requirements

The deadcode tool must be installed:

	$ go install golang.org/x/tools/cmd/deadcode@latest

# Multiple Go Modules

By default, all provided entrypoints must belong to the same Go module.
To analyze entrypoints from different modules, use the -filter flag with a
custom regular expression to specify which packages to analyze:

	$ deadmono -filter "github.com/myorg/.*" module1/main.go module2/main.go

# Limitations

The analysis inherits all limitations from the deadcode tool, including:
  - Valid only for a single GOOS/GOARCH/-tags configuration
  - Does not understand //go:linkname directives
  - Requires careful judgement before deleting reported functions

See https://pkg.go.dev/golang.org/x/tools/cmd/deadcode for more details
on the underlying analysis.
*/
package main
