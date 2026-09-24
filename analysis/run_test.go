package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/arxeiss/deadmono/analysis"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Runner", func() {
	var (
		stdOut *bytes.Buffer
		stdErr *bytes.Buffer
	)

	BeforeEach(func() {
		stdOut = bytes.NewBuffer(nil)
		stdErr = bytes.NewBuffer(nil)
	})

	It("fails on no main files in current directory", func() {
		ctx := context.Background()
		// Current directory contains only testdata with main files, which must be skipped.
		r := analysis.New(stdOut, stdErr, []string{})
		err := r.Run(ctx)
		Expect(err).To(MatchError("no main files found in '.'"))
	})

	DescribeTable("fails on invalid paths",
		func(path, expectedErr string) {
			ctx := context.Background()
			r := analysis.New(stdOut, stdErr, []string{path})
			err := r.Run(ctx)
			Expect(err).To(MatchError(HavePrefix(expectedErr)))
		},
		Entry("Non existing file", "testdata/nonexisting/main.go",
			"failed to access 'testdata/nonexisting/main.go': "),
		Entry("Non existing directory", "testdata/nonexisting/...",
			"failed to access 'testdata/nonexisting/...': "),
		Entry("File with recursive suffix", "testdata/cli/main.go/...",
			"'testdata/cli/main.go' is not a directory"),
		Entry("Directory without main files", "testdata/allinone/pkg",
			"no main files found in 'testdata/allinone/pkg'"),
	)

	It("fails on no Go module", func() {
		ctx := context.Background()
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o600)).
			To(Succeed())
		r := analysis.New(stdOut, stdErr, []string{dir})
		err := r.Run(ctx)
		Expect(err).To(MatchError(ContainSubstring(
			"failed to list dependencies: go: go.mod file not found in current directory or any parent directory",
		)))
	})

	It("fails on different go modules", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{
			"testdata/allinone/services/authn/main.go",
			"testdata/cli/main.go",
		})
		err := r.Run(ctx)
		Expect(err).To(MatchError("different modules are not supported without filter flag: " +
			"github.com/arxeiss/deadmono/sample/allinone/ != github.com/arxeiss/deadmono/sample/cli/"))
	})

	It("fails on invalid filter flag", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{"testdata/allinone/services/authn/main.go"})
		r.FilterFlag = "*"
		err := r.Run(ctx)
		Expect(err).To(MatchError(HavePrefix(
			//nolint:dupword // no duplicate word, but real error
			"failed to list deadcode: deadcode: -filter: error parsing regexp: missing argument to repetition operator",
		)))
	})

	DescribeTable("Verify all in one example",
		func(paths []string) {
			ctx := context.Background()

			expectedOutput := "pkg/cache/cache.go:12:6: unreachable func: Delete\n" +
				"pkg/crypto: unreachable package\n" +
				"pkg/logging/logging.go:12:6: unreachable func: Warn\n" +
				"pkg/logging/logging.go:6:6: unreachable func: Debug\n" +
				"services/authn/internal/auth.go:18:6: unreachable func: RunFromTest\n"

			r := analysis.New(stdOut, stdErr, paths)
			Expect(r.Run(ctx)).To(Succeed())
			Expect(stdOut.String()).To(Equal(expectedOutput))

			By("Running in reverse order")
			slices.Reverse(paths)
			stdOut.Reset()
			stdErr.Reset()

			r = analysis.New(stdOut, stdErr, paths)
			Expect(r.Run(ctx)).To(Succeed())
			Expect(stdOut.String()).To(Equal(expectedOutput))
		},
		Entry("Directory", []string{"testdata/allinone"}),
		Entry("Directory with trailing slash", []string{"testdata/allinone/"}),
		Entry("Recursive directory", []string{"testdata/allinone/..."}),
		Entry("Multiple directories", []string{
			"testdata/allinone/services/authn",
			"testdata/allinone/services/config/...",
			"testdata/allinone/services/healthcheck/main.go",
		}),
		Entry("Alphabetical", []string{
			"testdata/allinone/services/authn/main.go",
			"testdata/allinone/services/config/main.go",
			"testdata/allinone/services/healthcheck/main.go",
		}),
		Entry("Mixed 1", []string{
			"testdata/allinone/services/config/main.go",
			"testdata/allinone/services/authn/main.go",
			"testdata/allinone/services/healthcheck/main.go",
		}),
		Entry("Mixed 2", []string{
			"testdata/allinone/services/authn/main.go",
			"testdata/allinone/services/healthcheck/main.go",
			"testdata/allinone/services/config/main.go",
		}),
	)

	It("Verify scanning current directory without paths", func() {
		ctx := context.Background()
		wd, err := os.Getwd()
		Expect(err).To(Succeed())
		Expect(os.Chdir("testdata/allinone")).To(Succeed())
		DeferCleanup(os.Chdir, wd)

		r := analysis.New(stdOut, stdErr, nil)
		r.DebugFlag = true
		Expect(r.Run(ctx)).To(Succeed())
		Expect(stdErr.String()).To(HavePrefix("No paths provided, scanning current directory\n" +
			"Found main file: services/authn/main.go\n" +
			"Found main file: services/config/main.go\n" +
			"Found main file: services/healthcheck/main.go\n"))
		Expect(stdOut.String()).To(Equal("pkg/cache/cache.go:12:6: unreachable func: Delete\n" +
			"pkg/crypto: unreachable package\n" +
			"pkg/logging/logging.go:12:6: unreachable func: Warn\n" +
			"pkg/logging/logging.go:6:6: unreachable func: Debug\n" +
			"services/authn/internal/auth.go:18:6: unreachable func: RunFromTest\n"))
	})

	It("Verify debug output", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{"testdata/allinone/services/authn/main.go"})
		r.DebugFlag = true
		Expect(r.Run(ctx)).To(Succeed())

		dir, err := filepath.Abs("testdata/allinone/")
		Expect(err).To(Succeed())
		dir += "/"

		// Count dependencies ourselves. It can vary over time when Go is updated, so we don't want to hardcode it.
		cmd := exec.CommandContext(ctx, "go", "list", "-f", `{{range .Deps}}{{.}}{{"\n"}}{{end}}`)
		cmd.Dir = filepath.Dir("testdata/allinone/services/authn/main.go")
		out, err := cmd.CombinedOutput()
		Expect(err).To(Succeed())
		depCount := strings.Count(string(out), "\n")

		Expect(stdErr.String()).To(HavePrefix(
			"Start scanning entrypoint: " + dir + "services/authn/main.go\n" +
				"Detected module name: github.com/arxeiss/deadmono/sample/allinone/\n" +
				"Detected " + strconv.Itoa(depCount) + " dependencies\n" +
				"Detected root path: " + dir + "\n" +
				"Starting to scan " + dir + "services/authn for deadcode, might take a while\n" +
				"Scanning " + dir + "services/authn for deadcode finished in ",
		))
	})

	DescribeTable("Verify properly passing flags",
		func(flagSetter func(r *analysis.Runner), expectedOutput string) {
			ctx := context.Background()

			r := analysis.New(stdOut, stdErr, []string{"testdata/allinone/services/authn/main.go"})
			flagSetter(r)

			Expect(r.Run(ctx)).To(Succeed())
			Expect(stdOut.String()).To(Equal(expectedOutput))
		},
		Entry("Generated", func(r *analysis.Runner) {
			r.GeneratedFlag = true
		}, "pkg/cache: unreachable package\n"+
			"pkg/crypto: unreachable package\n"+
			"pkg/http/http.go:13:6: unreachable func: Post\n"+
			"pkg/http/http.go:17:6: unreachable func: Put\n"+
			"pkg/http/http.go:9:6: unreachable func: Get\n"+
			"pkg/logging/logging.go:12:6: unreachable func: Warn\n"+
			"pkg/logging/logging.go:6:6: unreachable func: Debug\n"+
			"services/authn/internal/auth.go:18:6: unreachable func: RunFromTest\n"+
			"services/authn/internal/generated.go:5:6: unreachable func: Generated\n",
		),
		Entry("Tests", func(r *analysis.Runner) {
			r.TestFlag = true
		}, "pkg/cache: unreachable package\n"+
			"pkg/crypto: unreachable package\n"+
			"pkg/http/http.go:17:6: unreachable func: Put\n"+
			"pkg/http/http.go:9:6: unreachable func: Get\n"+
			"pkg/logging/logging.go:12:6: unreachable func: Warn\n",
		),
		Entry("Tags", func(r *analysis.Runner) {
			r.TagsFlag = "rpi"
		}, "pkg/cache: unreachable package\n"+
			"pkg/crypto: unreachable package\n"+
			"pkg/http/http.go:17:6: unreachable func: Put\n"+
			"pkg/logging/logging.go:12:6: unreachable func: Warn\n"+
			"pkg/logging/logging.go:6:6: unreachable func: Debug\n"),
		Entry("No dead pkg", func(r *analysis.Runner) {
			r.NoDeadPkgFlag = true
		}, "pkg/http/http.go:13:6: unreachable func: Post\n"+
			"pkg/http/http.go:17:6: unreachable func: Put\n"+
			"pkg/http/http.go:9:6: unreachable func: Get\n"+
			"pkg/logging/logging.go:12:6: unreachable func: Warn\n"+
			"pkg/logging/logging.go:6:6: unreachable func: Debug\n"+
			"services/authn/internal/auth.go:18:6: unreachable func: RunFromTest\n"),
	)

	It("Handles properly multiple modules with filter", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{
			"testdata/allinone/services/config/main.go",
			"testdata/cli/main.go",
		})
		r.FilterFlag = "Masterminds/semver|allinone"
		Expect(r.Run(ctx)).To(Succeed())

		lines := strings.Split(strings.TrimSpace(stdOut.String()), "\n")
		foundStrictNewVersion, foundMustParse := "", ""
		for _, line := range lines {
			if strings.Contains(line, "StrictNewVersion") {
				foundStrictNewVersion = line
			} else if strings.Contains(line, "MustParse") {
				foundMustParse = line
			}
			Expect(line).To(SatisfyAny(
				ContainSubstring("github.com/!masterminds/semver/v3@"),
				ContainSubstring("deadmono/analysis/testdata/allinone/pkg/cache/"),
				ContainSubstring("deadmono/analysis/testdata/allinone/pkg/logging/"),
			))
		}
		Expect(foundStrictNewVersion).To(BeEmpty(), "StrictNewVersion should not be found")
		Expect(foundMustParse).To(BeEmpty(), "MustParse should not be found")
	})

	It("Handles JSON output", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{
			"testdata/allinone/services/authn/main.go",
			"testdata/allinone/services/config/main.go",
			"testdata/allinone/services/healthcheck/main.go",
		})
		r.JSONFlag = true
		r.GeneratedFlag = true
		Expect(r.Run(ctx)).To(Succeed())

		expected := []*analysis.Package{
			{
				Name: "cache",
				Path: "github.com/arxeiss/deadmono/sample/allinone/pkg/cache",
				Funcs: []*analysis.Function{
					{
						Name: "Delete",
						Position: analysis.Position{
							File: "pkg/cache/cache.go",
							Line: 12,
							Col:  6,
						},
					},
				},
			},
			{
				Name:         "crypto",
				Path:         "github.com/arxeiss/deadmono/sample/allinone/pkg/crypto",
				WholePackage: true,
				Funcs:        make([]*analysis.Function, 0),
			},
			{
				Name: "logging",
				Path: "github.com/arxeiss/deadmono/sample/allinone/pkg/logging",
				Funcs: []*analysis.Function{
					{
						Name: "Debug",
						Position: analysis.Position{
							File: "pkg/logging/logging.go",
							Line: 6,
							Col:  6,
						},
					},
					{
						Name: "Warn",
						Position: analysis.Position{
							File: "pkg/logging/logging.go",
							Line: 12,
							Col:  6,
						},
					},
				},
			},
			{
				Name: "internal",
				Path: "github.com/arxeiss/deadmono/sample/allinone/services/authn/internal",
				Funcs: []*analysis.Function{
					{
						Name: "RunFromTest",
						Position: analysis.Position{
							File: "services/authn/internal/auth.go",
							Line: 18,
							Col:  6,
						},
					},
					{
						Name: "Generated",
						Position: analysis.Position{
							File: "services/authn/internal/generated.go",
							Line: 5,
							Col:  6,
						},
						Generated: true,
					},
				},
			},
		}

		expectedOut, err := json.MarshalIndent(expected, "", "\t")
		Expect(err).To(Succeed())
		Expect(stdOut.Bytes()).To(MatchJSON(expectedOut))
	})

	It("Handles JSON output with different modules", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{
			"testdata/allinone/services/config/main.go",
			"testdata/cli/main.go",
		})
		r.JSONFlag = true
		r.FilterFlag = "Masterminds/semver"
		Expect(r.Run(ctx)).To(Succeed())

		var out []*analysis.Package
		err := json.Unmarshal(stdOut.Bytes(), &out)
		Expect(err).To(Succeed())

		Expect(out).To(HaveLen(1))
		Expect(out[0].Path).To(Equal("github.com/Masterminds/semver/v3"))
		Expect(out[0].Name).To(Equal("semver"))

		Expect(out[0].Funcs).To(ContainElement(PointTo(MatchAllFields(Fields{
			"Name": Equal("NewConstraint"),
			"Position": MatchAllFields(Fields{
				"File": ContainSubstring("github.com/!masterminds/semver/v3@"),
				"Line": BeNumerically(">", 0),
				"Col":  BeNumerically(">", 0),
			}),
			"Generated": BeFalse(),
			"Marker":    BeFalse(),
		}))))

		Expect(out[0].Funcs).NotTo(ContainElement(PointTo(MatchAllFields(Fields{
			"Name": Equal("StrictNewVersion"),
		}))))
		Expect(out[0].Funcs).NotTo(ContainElement(PointTo(MatchAllFields(Fields{
			"Name": Equal("MustParse"),
		}))))
	})
})
