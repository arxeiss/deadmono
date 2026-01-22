package analysis

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type (
	Runner struct {
		writer    io.Writer
		errWriter io.Writer
		paths     []string

		lastModule string
		// DebugFlag turns on more verbose output.
		DebugFlag bool
		// GeneratedFlag turns on reporting of dead functions in generated Go files.
		GeneratedFlag bool
		// TestFlag turns on reporting of dead functions in test files.
		TestFlag bool
		// TagsFlag is a comma-separated list of extra build tags.
		TagsFlag string
		// JSONFlag turns on JSONFlag output.
		JSONFlag bool
	}

	entrypointInfo struct {
		deps     map[string]struct{}
		deadCode map[string]map[string]struct{}
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
	if err := r.verifyBinaries(ctx); err != nil {
		return err
	}

	eps := make([]*entrypointInfo, 0, len(r.paths))
	for _, path := range r.paths {
		ep, err := r.scanEntrypoint(ctx, path)
		if err != nil {
			return err
		}
		eps = append(eps, ep)
	}

	deadCode := r.intersectDeadcode(eps)
	if r.JSONFlag {
		return fmt.Errorf("JSON output is not implemented yet")
	}

	allPaths := make([]string, 0)
	for _, lines := range deadCode {
		for line := range lines {
			allPaths = append(allPaths, line)
		}
	}

	slices.Sort(allPaths)
	for _, path := range allPaths {
		fmt.Fprintln(r.writer, path)
	}

	return nil
}

func (r *Runner) scanEntrypoint(ctx context.Context, path string) (*entrypointInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert '%s' to absolute path: %w", path, err)
	}
	r.writeDebug("Start scanning entrypoint: %s", absPath)

	if err := r.verifyModule(ctx, absPath); err != nil {
		return nil, err
	}

	ep, err := r.listDependencies(ctx, absPath)
	if err != nil {
		return nil, err
	}

	ep.deadCode, err = r.listEntrypointDeadcode(ctx, absPath)
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

	moduleName := strings.TrimSuffix(strings.TrimSpace(string(out)), "/") + "/"
	if r.lastModule != "" && r.lastModule != moduleName {
		return fmt.Errorf("different modules in one run are not supported: %s != %s", r.lastModule, moduleName)
	}
	r.lastModule = moduleName
	r.writeDebug("Detected module name: %s", r.lastModule)
	return nil
}

func (r *Runner) listDependencies(ctx context.Context, absPath string) (*entrypointInfo, error) {
	out, err := getCommandOutput(ctx, filepath.Dir(absPath), "go", "list", "-f", `{{range .Deps}}{{.}}{{"\n"}}{{end}}`)
	if err != nil {
		return nil, fmt.Errorf("failed to list dependencies: %w", err)
	}

	ep := &entrypointInfo{
		deps: make(map[string]struct{}),
	}
	for _, line := range strings.Split(string(out), "\n") {
		if m, found := strings.CutPrefix(line, r.lastModule); found {
			ep.deps[m] = struct{}{}
		}
	}
	r.writeDebug("Detected %d dependencies", len(ep.deps))
	return ep, nil
}

func (r *Runner) listEntrypointDeadcode(ctx context.Context, absPath string) (map[string]map[string]struct{}, error) {
	absDirPath := filepath.Dir(absPath)
	out, err := getCommandOutput(ctx, absDirPath, "go", "list", "-f", `{{.Root}}`)
	if err != nil {
		return nil, fmt.Errorf("Failed to list root path: %w", err)
	}
	rootPath := filepath.Clean(strings.TrimSpace(string(out))) + string(filepath.Separator)
	r.writeDebug("Detected root path: %s", rootPath)

	r.writeDebug("Starting to scan %s for deadcode, might take a while", absDirPath)
	timeStart := time.Now()
	args := []string{"-f", `{{range .Funcs}}{{printf "%s: unreachable func: %s\n" .Position .Name}}{{end}}`}
	if r.GeneratedFlag {
		args = append(args, "-generated")
	}
	if r.TestFlag {
		args = append(args, "-test")
	}
	if r.TagsFlag != "" {
		args = append(args, "-tags", r.TagsFlag)
	}
	out, err = getCommandOutput(ctx, absDirPath, "deadcode", append(args, "./...")...)
	if err != nil {
		return nil, fmt.Errorf("Failed to list deadcode: %w", err)
	}
	r.writeDebug("Scanning %s for deadcode finished in %s", absDirPath, time.Since(timeStart))

	deadCode := map[string]map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		line, found := strings.CutPrefix(line, rootPath)
		if !found {
			line, _ = strings.CutPrefix(filepath.Join(absDirPath, line), rootPath)
		}
		filePath, _, found := strings.Cut(line, ":")
		if !found {
			continue
		}

		pkg := filepath.Dir(filePath)
		if deadCode[pkg] == nil {
			deadCode[pkg] = make(map[string]struct{})
		}
		deadCode[pkg][line] = struct{}{}
	}
	return deadCode, nil
}

func (r *Runner) intersectDeadcode(eps []*entrypointInfo) map[string]map[string]struct{} {
	result := eps[0]
	for pkg := range result.deps {
		if _, found := result.deadCode[pkg]; !found {
			// If pkg is using all functions inside (has no records in deadcode), we should mark that to result.
			// This is special case, but we need to keep track of it.
			result.deps[pkg] = struct{}{}
			result.deadCode[pkg] = make(map[string]struct{})
		}
	}

	for _, ep := range eps[1:] {
		for pkg, lines := range ep.deadCode {
			// Iterating entrypoint uses package, that wasn't scanned yet,
			// just add all what is unused in iterating entrypoint.
			_, visitedDep := result.deps[pkg]
			// Check deadcode in result already, if not exists too, same situation as above
			resultDeadLines, exists := result.deadCode[pkg]

			if !exists || !visitedDep {
				result.deps[pkg] = struct{}{}
				result.deadCode[pkg] = lines
				continue
			}

			// Package was already scanned, so we need to intersect the deadcode.
			// Only keep lines that appear in both maps.
			intersection := make(map[string]struct{})
			for line := range resultDeadLines {
				// If what we consider unused so far exists in current entrypoint too, keep it.
				if _, found := lines[line]; found {
					intersection[line] = struct{}{}
				}
			}
			result.deadCode[pkg] = intersection
		}

		for pkg := range ep.deps {
			// If our pkg is using all functions inside (has no records in deadcode), we should mark that to result.
			// This is special case, but we need to keep track of it.
			if len(ep.deadCode[pkg]) == 0 {
				result.deps[pkg] = struct{}{}
				result.deadCode[pkg] = make(map[string]struct{})
			}
		}
	}
	return result.deadCode
}
