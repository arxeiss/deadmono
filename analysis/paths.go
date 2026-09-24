package analysis

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// resolvePaths expands provided paths into a list of Go files containing main function.
// Paths to files are kept as they are. Paths to directories (optionally with "/..." suffix) are scanned recursively.
// If no path is provided, current directory is scanned.
func (r *Runner) resolvePaths() ([]string, error) {
	if len(r.paths) == 0 {
		r.writeDebug("No paths provided, scanning current directory")
		return r.findMainFiles(".")
	}

	resolved := make([]string, 0, len(r.paths))
	for _, path := range r.paths {
		dir, isRecursive := strings.CutSuffix(path, "/...")
		if isRecursive && dir == "" {
			dir = "/"
		}
		info, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to access '%s': %w", path, err)
		}
		if !isRecursive && !info.IsDir() {
			resolved = append(resolved, path)
			continue
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("'%s' is not a directory", dir)
		}

		mainFiles, err := r.findMainFiles(dir)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, mainFiles...)
	}
	return resolved, nil
}

// findMainFiles recursively walks the directory and returns all Go files containing main function.
// It follows the same rules as "./..." pattern of Go tooling, so it skips "testdata" and "vendor" directories
// and directories starting with "." or "_". Only the first found main file per directory is returned.
func (r *Runner) findMainFiles(root string) ([]string, error) {
	mainFiles := make([]string, 0)
	seenDirs := make(map[string]struct{})
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "testdata" || name == "vendor" ||
				strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		// Multiple files in one directory can declare main function (for example with different build tags).
		// All of them represent the same entrypoint, so scan the directory only once.
		if _, seen := seenDirs[filepath.Dir(path)]; seen {
			return nil
		}

		hasMain, err := hasMainFunc(fset, path)
		if err != nil {
			return err
		}
		if hasMain {
			r.writeDebug("Found main file: %s", path)
			mainFiles = append(mainFiles, path)
			seenDirs[filepath.Dir(path)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan '%s' for main files: %w", root, err)
	}
	if len(mainFiles) == 0 {
		return nil, fmt.Errorf("no main files found in '%s'", root)
	}
	return mainFiles, nil
}

func hasMainFunc(fset *token.FileSet, path string) (bool, error) {
	// Parse package clause first, to avoid parsing whole file for non-main packages.
	f, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
	if err != nil {
		return false, fmt.Errorf("failed to parse '%s': %w", path, err)
	}
	if f.Name.Name != "main" {
		return false, nil
	}

	f, err = parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return false, fmt.Errorf("failed to parse '%s': %w", path, err)
	}
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "main" {
			return true, nil
		}
	}
	return false, nil
}
