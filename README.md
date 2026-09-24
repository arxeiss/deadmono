<p align="center"><img src="./deadmono-small.png" width="400"></p>

# deadmono

[![Lint and test](https://github.com/arxeiss/deadmono/actions/workflows/lint-test.yaml/badge.svg)](https://github.com/arxeiss/deadmono/actions/workflows/lint-test.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/arxeiss/deadmono.svg)](https://pkg.go.dev/github.com/arxeiss/deadmono)
[![codecov](https://codecov.io/gh/arxeiss/deadmono/graph/badge.svg?token=UDEWAC3KC2)](https://codecov.io/gh/arxeiss/deadmono)

**Dead code detection for Go monorepos**

`deadmono` reports unreachable functions across multiple entrypoints in Go monorepos. It extends the functionality of the [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) tool to work with monorepos containing multiple main packages.

Usage is not strictly limited to monorepos. It can be used in any Go project with multiple entrypoints.
For example, you can use it to detect dead code in internal SDK shared across many projects. For that usecase,
you must use the `-filter` flag.

## Installation

```bash
# Install deadmono
go install github.com/arxeiss/deadmono/cmd/deadmono@latest

# Install the required deadcode tool
go install golang.org/x/tools/cmd/deadcode@latest
```

## Usage

```bash
deadmono [flags] [path/to/main.go | path/to/dir | path/to/dir/...] ...
```

### Example

Analyze three services in a monorepo:

```bash
deadmono services/authn/main.go services/config/main.go services/healthcheck/main.go
```

This will report functions that are unused by all three services.

Instead of listing main files manually, you can pass a directory (optionally with `/...` suffix).
It is scanned recursively for Go files declaring `func main`, skipping `testdata`, `vendor` and
directories starting with `.` or `_`. Without any argument, the current directory is scanned:

```bash
deadmono services/...
deadmono services
deadmono
```

### Flags

- `-test` - Analyze test executables too (passed to deadcode)
- `-generated` - Include dead functions in generated Go files (passed to deadcode)
- `-tags string` - Comma-separated list of build tags (passed to deadcode)
- `-filter string` - Filter packages by regular expression (passed to deadcode). Default: `<module>` (filters to the module of the first entrypoint)
- `-json` - Output results in JSON format (same format as deadcode)
- `-no-dead-pkg` - Don't report unreachable packages, keeps the output compatible with deadcode
- `-debug` - Enable verbose debug output
- `-help` - Show help message

### Output

The output format matches `deadcode`, with two differences:

**File paths.** Since `deadmono` analyzes multiple entrypoints, it uses a consistent path strategy:

- **Single module** - Paths relative to `go.mod` when all entrypoints are in the same module
- **Multiple modules** - Absolute paths when entrypoints span different modules

**Unreachable packages.** `deadmono` also reports whole packages that no entrypoint imports. This has no
counterpart in `deadcode`, see [Dead package detection](#dead-package-detection) below.

## Dead package detection

`deadmono` runs `deadcode` once per entrypoint, from that entrypoint's own directory. A package which no
entrypoint imports therefore never appears in any of those runs, so the package based intersection has nothing
to intersect and would silently skip it - even though such a package is completely dead.

To close that gap, `deadmono` lists all packages of the module and reports those which are not imported by any
of the analyzed entrypoints. Instead of listing every function separately, the package is reported as a whole:

```
pkg/crypto: unreachable package
```

In JSON output, such a package record has the extra `WholePackage` field set and an empty list of functions:

```json
{
  "Name": "crypto",
  "Path": "github.com/myorg/monorepo/pkg/crypto",
  "WholePackage": true,
  "Funcs": []
}
```

`main` packages are never reported this way, as they are entrypoints and are not expected to be imported.

A single `deadcode ./...` run over the whole module does see such packages, but it reports them function by
function and cannot do the per service intersection `deadmono` is built for.

### When it is enabled

Dead package detection is **enabled by default**, but only when all entrypoints belong to the same Go module.
When entrypoints span [multiple Go modules](#multiple-go-modules), there is no single module to enumerate the
packages from, so the detection is skipped automatically.

### Compatibility with `deadcode`

> [!IMPORTANT]
> With dead package detection turned on - which is the default - the output is **not compatible** with
> `deadcode`. Text output contains `<package>: unreachable package` lines instead of the usual
> `file:line:col: unreachable func: <name>` lines for each function of that package, and JSON output contains
> package records with the extra `WholePackage` field and an empty `Funcs` list.

If you feed the output into tooling which expects the `deadcode` format, use the `-no-dead-pkg` flag to
suppress unreachable packages and keep the output fully compatible.

## Requirements

- The [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) tool must be installed

### Outdated `deadcode` tool

If you see an error like this:

```
This application uses version go1.26 of the source-processing packages but runs version go1.27 of 'go list'.
It may fail to process source files that rely on newer language features. If so, rebuild the application using a newer version of Go.
```

it is not an issue of `deadmono`, but of an outdated `deadcode` binary built with an older Go version.
Reinstall it with your current Go toolchain:

```bash
go install golang.org/x/tools/cmd/deadcode@latest
```

## Multiple Go Modules

By default, all provided entrypoints must belong to the same Go module. To analyze entrypoints from different modules, use the `-filter` flag with a custom regular expression:

```bash
# Analyze services from different modules
deadmono -filter "github.com/myorg/.*" module1/services/api/main.go module2/services/worker/main.go
```

When using a custom `-filter` flag, deadmono supports analyzing entrypoints across multiple Go modules.


## The Problem

In a monorepo with multiple services sharing common packages, traditional dead code analysis tools like `deadcode` analyze each service independently. This creates major issue

### Simple Intersection Doesn't Work

You might think: "Just run deadcode on each service and only delete functions reported as dead in ALL services." However, this fails when services don't import the same packages:

```
Monorepo Structure:
├── services/
│   ├── service-a/     (imports pkg/cache, pkg/logging)
│   ├── service-b/     (imports pkg/cache, pkg/logging)
│   └── service-c/     (imports pkg/logging only - NO pkg/cache!)
└── pkg/
    ├── cache/
    │   ├── Get()      ✓ Used by service-a
    │   ├── Set()      ✓ Used by service-b
    │   └── Delete()   ✗ Actually dead (unused by anyone)
    └── logging/
        └── Info()

Running deadcode on each service:
  service-a: Reports cache.Set(), cache.Delete() as dead
  service-b: Reports cache.Get(), cache.Delete() as dead
  service-c: Reports NOTHING about pkg/cache (doesn't import it!)

Simple intersection (dead in ALL services):
  Result: EMPTY SET ❌

  Why? service-c doesn't import pkg/cache at all, so it doesn't
  report anything about it. The intersection finds nothing, even
  though cache.Delete() is genuinely unused by all services!
```

## The Solution

`deadmono` solves the issue with **package-based intersection**:

1. **Tracks which packages each service imports** - Knows which services should have an opinion about a package
2. **Analyzes each entrypoint separately** - Runs deadcode on each service
3. **Intersects results per package** - Only considers services that actually import each package
4. **Reports truly dead functions** - Functions unreachable from ALL services that import their package
5. **Reports truly dead packages** - Packages which are not imported by any service at all, see
   [Dead package detection](#dead-package-detection)

### How It Works on the Example Above

```
Running deadmono on all three services:

For pkg/cache (imported by service-a, service-b):
  service-a reports: cache.Set(), cache.Delete() as dead
  service-b reports: cache.Get(), cache.Delete() as dead
  service-c: IGNORED (doesn't import pkg/cache)

  Intersection (service-a ∩ service-b):
    ✓ cache.Delete() - dead in BOTH services that import it

For pkg/logging (imported by service-a, service-b, service-c):
  All services use logging.Info()

  Intersection (service-a ∩ service-b ∩ service-c):
    (empty - no dead functions)

Final Result:
  pkg/cache/cache.go:X:X: unreachable func: Delete
```

This way, you can confidently identify and remove truly unused code in your monorepo, without false positives from services that don't even import a package.

## Limitations

The analysis inherits all limitations from the `deadcode` tool, including:

- Valid only for a single GOOS/GOARCH/-tags configuration
- Does not understand `//go:linkname` directives
- Requires careful judgement before deleting reported functions

See the [deadcode documentation](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) for more details on the underlying analysis.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Acknowledgments

This tool builds upon the excellent [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) tool from the Go team.
