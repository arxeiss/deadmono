package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type (
	// Runner specify all configuration for running deadcode analysis across monorepo.
	Runner struct {
		writer    io.Writer
		errWriter io.Writer

		// TagsFlag is a comma-separated list of extra build tags.
		TagsFlag string
		// FilterFlag is a regular expression to filter packages by.
		FilterFlag string

		commonModule    string
		paths           []string
		hasCommonModule bool

		// DebugFlag turns on more verbose output.
		DebugFlag bool
		// GeneratedFlag turns on reporting of dead functions in generated Go files.
		GeneratedFlag bool
		// TestFlag turns on reporting of dead functions in test files.
		TestFlag bool
		// JSONFlag turns on JSONFlag output.
		JSONFlag bool
	}

	entrypointInfo struct {
		deps     map[string]struct{}
		deadCode map[string]deadPackageFuncs
		absPath  string
	}

	deadPackageFuncs struct {
		pkg   *Package
		funcs map[string]*Function
	}
)

// New creates runner for analysis.
// Pass paths to all Go main files within monorepo. If you pass only 1 path, it will behave like normal deadcode.
func New(writer, errWriter io.Writer, paths []string) *Runner {
	return &Runner{
		writer:    writer,
		errWriter: errWriter,
		paths:     paths,
	}
}

func (r *Runner) writeStderr(format string, args ...any) {
	fmt.Fprintf(r.errWriter, strings.TrimSuffix(format, "\n")+"\n", args...)
}

func (r *Runner) writeDebug(format string, args ...any) {
	if r.DebugFlag {
		r.writeStderr(format, args...)
	}
}

// Run the deadcode analysis across monorepo and prints out unused exported functions.
func (r *Runner) Run(ctx context.Context) error {
	if len(r.paths) == 0 {
		return fmt.Errorf("no paths provided")
	}
	err := r.verifyBinaries(ctx)
	if err != nil {
		return err
	}

	// Collect as much information as possible about entrypoints before we start scanning for deadcode.
	eps := make([]*entrypointInfo, 0, len(r.paths))
	for _, path := range r.paths {
		var ep *entrypointInfo
		ep, err = r.scanEntrypoint(ctx, path)
		if err != nil {
			return err
		}
		eps = append(eps, ep)
	}

	// Scan for deadcode.
	for _, ep := range eps {
		ep.deadCode, err = r.listEntrypointDeadCode(ctx, ep.absPath)
		if err != nil {
			return err
		}
	}

	deadCode := r.intersectDeadCode(eps)

	if r.JSONFlag {
		return r.printJSON(ctx, deadCode)
	}
	r.printText(ctx, deadCode)

	return nil
}

func (r *Runner) scanEntrypoint(ctx context.Context, path string) (*entrypointInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert '%s' to absolute path: %w", path, err)
	}
	r.writeDebug("Start scanning entrypoint: %s", absPath)

	err = r.verifyModule(ctx, absPath)
	if err != nil {
		return nil, err
	}

	ep, err := r.listDependencies(ctx, absPath)
	if err != nil {
		return nil, err
	}

	return ep, nil
}

func (r *Runner) verifyBinaries(_ context.Context) error {
	_, err := exec.LookPath("deadcode")
	if err != nil {
		r.writeStderr("Install deadcode with 'go install golang.org/x/tools/cmd/deadcode@latest'")
		return err
	}
	_, err = exec.LookPath("go")
	if err != nil {
		r.writeStderr("Go is not in $PATH")
		return err
	}
	return nil
}

func (r *Runner) verifyModule(ctx context.Context, absPath string) error {
	// Verify module name
	out, err := getCommandOutput(ctx, filepath.Dir(absPath), "go", "list", "-m")
	if err != nil {
		return fmt.Errorf("failed to list module name: %w", err)
	}

	m := strings.TrimSuffix(strings.TrimSpace(string(out)), "/") + "/"
	switch {
	case r.commonModule == "":
		r.commonModule = m
		r.hasCommonModule = true
	case r.commonModule == m:
		// Do nothing, as everything was set before
	case r.FilterFlag == "<module>" || r.FilterFlag == "":
		// If we have custom filter, we don't need to have same module, as we will filter by regexp anyway.
		// If flag is <module>, we need all scans to be within same module, otherwise intersection will be empty.
		return fmt.Errorf("different modules are not supported without filter flag: %s != %s", r.commonModule, m)
	default:
		r.hasCommonModule = false
	}
	r.writeDebug("Detected module name: %s", r.commonModule)
	return nil
}

func (r *Runner) listDependencies(ctx context.Context, absPath string) (*entrypointInfo, error) {
	out, err := getCommandOutput(ctx, filepath.Dir(absPath), "go", "list", "-f", `{{range .Deps}}{{.}}{{"\n"}}{{end}}`)
	if err != nil {
		return nil, fmt.Errorf("failed to list dependencies: %w", err)
	}

	ep := &entrypointInfo{
		absPath: absPath,
		deps:    make(map[string]struct{}),
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		ep.deps[line] = struct{}{}
	}
	r.writeDebug("Detected %d dependencies", len(ep.deps))
	return ep, nil
}

func (r *Runner) getDeadCodeArgs() []string {
	args := []string{"-json"}
	if r.GeneratedFlag {
		args = append(args, "-generated")
	}
	if r.TestFlag {
		args = append(args, "-test")
	}
	if r.TagsFlag != "" {
		args = append(args, "-tags", r.TagsFlag)
	}
	if r.FilterFlag != "" {
		args = append(args, "-filter", r.FilterFlag)
	}
	return append(args, "./...")
}

func (r *Runner) listEntrypointDeadCode(ctx context.Context, absPath string) (map[string]deadPackageFuncs, error) {
	absDirPath := filepath.Dir(absPath)
	out, err := getCommandOutput(ctx, absDirPath, "go", "list", "-f", `{{.Root}}`)
	if err != nil {
		return nil, fmt.Errorf("failed to list root path: %w", err)
	}
	rootPath := filepath.Clean(strings.TrimSpace(string(out))) + string(filepath.Separator)
	r.writeDebug("Detected root path: %s", rootPath)

	r.writeDebug("Starting to scan %s for deadcode, might take a while", absDirPath)
	timeStart := time.Now()

	out, err = getCommandOutput(ctx, absDirPath, "deadcode", r.getDeadCodeArgs()...)
	if err != nil {
		return nil, fmt.Errorf("failed to list deadcode: %w", err)
	}
	r.writeDebug("Scanning %s for deadcode finished in %s", absDirPath, time.Since(timeStart))

	deadCode := map[string]deadPackageFuncs{}
	pkgs := make([]*Package, 0)
	if err := json.Unmarshal(out, &pkgs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deadcode output: %w", err)
	}
	for _, pkg := range pkgs {
		dpf := deadPackageFuncs{
			pkg:   pkg,
			funcs: make(map[string]*Function),
		}
		for _, fun := range pkg.Funcs {
			f := fun.Position.File
			// If path is not absolute, it means it is relative to the module root.
			// Because we have many entrypoints, we need to make sure we have absolute paths.
			if !filepath.IsAbs(f) {
				f = filepath.Join(absDirPath, f)
			}
			// If all entrypoints are within same module, we can remove the path to module root from the file path.
			// Then it will be relative to go.mod file.
			if r.hasCommonModule {
				f, _ = strings.CutPrefix(f, rootPath)
			}
			fun.Position.File = f // Override back, so we have consistent output.
			dpf.funcs[fun.Name] = fun
		}
		pkg.Funcs = nil // Clear just not to use it accidentally.
		deadCode[pkg.Path] = dpf
	}

	return deadCode, nil
}

func (*Runner) intersectDeadCode(eps []*entrypointInfo) map[string]deadPackageFuncs {
	result := eps[0]
	for pkg := range result.deps {
		if _, found := result.deadCode[pkg]; !found {
			// If pkg is using all functions inside (has no records in deadcode), we should mark that to result.
			// This is special case, but we need to keep track of it.
			result.deps[pkg] = struct{}{}
			result.deadCode[pkg] = deadPackageFuncs{}
		}
	}

	for _, ep := range eps[1:] {
		for pkg, dpf := range ep.deadCode {
			// Iterating entrypoint uses package, that wasn't scanned yet,
			// just add all what is unused in iterating entrypoint.
			_, visitedDep := result.deps[pkg]
			// Check deadcode in result already, if not exists too, same situation as above
			resultDpf, exists := result.deadCode[pkg]

			if !exists || !visitedDep {
				result.deps[pkg] = struct{}{}
				result.deadCode[pkg] = dpf
				continue
			}

			// Package was already scanned, so we need to intersect the deadcode.
			// Only keep functions that appear in both maps.
			intersection := deadPackageFuncs{
				pkg:   resultDpf.pkg,
				funcs: make(map[string]*Function),
			}
			for fnName := range resultDpf.funcs {
				// If what we consider unused so far exists in current entrypoint too, keep it.
				if fun, found := dpf.funcs[fnName]; found {
					intersection.funcs[fnName] = fun
				}
			}
			result.deadCode[pkg] = intersection
		}

		for pkg := range ep.deps {
			// If our pkg is using all functions inside (has no records in deadcode), we should mark that to result.
			// This is special case, but we need to keep track of it.
			if len(ep.deadCode[pkg].funcs) == 0 {
				result.deps[pkg] = struct{}{}
				result.deadCode[pkg] = deadPackageFuncs{}
			}
		}
	}
	return result.deadCode
}

func (r *Runner) printJSON(_ context.Context, deadCode map[string]deadPackageFuncs) error {
	out := make([]*Package, 0, len(deadCode))

	for _, dpf := range deadCode {
		if len(dpf.funcs) == 0 {
			continue
		}
		dpf.pkg.Funcs = make([]*Function, 0, len(dpf.funcs))
		for _, fun := range dpf.funcs {
			dpf.pkg.Funcs = append(dpf.pkg.Funcs, fun)
		}
		slices.SortFunc(dpf.pkg.Funcs, func(a, b *Function) int {
			s := strings.Compare(a.Position.File, b.Position.File)
			if s != 0 {
				return s
			}
			return a.Position.Line - b.Position.Line
		})

		out = append(out, dpf.pkg)
	}
	slices.SortFunc(out, func(a, b *Package) int {
		return strings.Compare(a.Path, b.Path)
	})

	enc := json.NewEncoder(r.writer)
	enc.SetIndent("", "\t")
	return enc.Encode(out)
}

func (r *Runner) printText(_ context.Context, deadCode map[string]deadPackageFuncs) {
	allPaths := make([]string, 0)
	for _, dpf := range deadCode {
		for _, fun := range dpf.funcs {
			allPaths = append(allPaths, fmt.Sprintf(
				"%s:%d:%d: unreachable func: %s",
				fun.Position.File, fun.Position.Line, fun.Position.Col, fun.Name,
			))
		}
	}

	slices.Sort(allPaths)
	for _, path := range allPaths {
		fmt.Fprintln(r.writer, path)
	}
}
