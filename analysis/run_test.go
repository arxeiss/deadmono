package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
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

	It("fails on no paths", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{})
		err := r.Run(ctx)
		Expect(err).To(MatchError("no paths provided"))
	})

	It("fails on no Go module", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{"/home"})
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
			"github.com/arxeiss/deadmono/ != github.com/arxeiss/deadmono/testdata/"))
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

			expectedOutput := "analysis/testdata/allinone/pkg/cache/cache.go:12:6: unreachable func: Delete\n" +
				"analysis/testdata/allinone/pkg/logging/logging.go:12:6: unreachable func: Warn\n" +
				"analysis/testdata/allinone/pkg/logging/logging.go:6:6: unreachable func: Debug\n" +
				"analysis/testdata/allinone/services/authn/internal/auth.go:18:6: unreachable func: RunFromTest\n"

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

	It("Verify debug output", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{"testdata/allinone/services/authn/main.go"})
		r.DebugFlag = true
		Expect(r.Run(ctx)).To(Succeed())

		absPath, err := filepath.Abs(".")
		Expect(err).To(Succeed())
		dir := filepath.Dir(absPath) + "/"
		Expect(stdErr.String()).To(HavePrefix(
			"Start scanning entrypoint: " + dir + "analysis/testdata/allinone/services/authn/main.go\n" +
				"Detected module name: github.com/arxeiss/deadmono/\n" +
				"Detected 30 dependencies\n" +
				"Detected root path: " + dir + "\n" +
				"Starting to scan " + dir + "analysis/testdata/allinone/services/authn for deadcode, might take a while\n" +
				"Scanning " + dir + "analysis/testdata/allinone/services/authn for deadcode finished in ",
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
		}, "analysis/testdata/allinone/pkg/http/http.go:13:6: unreachable func: Post\n"+
			"analysis/testdata/allinone/pkg/http/http.go:17:6: unreachable func: Put\n"+
			"analysis/testdata/allinone/pkg/http/http.go:9:6: unreachable func: Get\n"+
			"analysis/testdata/allinone/pkg/logging/logging.go:12:6: unreachable func: Warn\n"+
			"analysis/testdata/allinone/pkg/logging/logging.go:6:6: unreachable func: Debug\n"+
			"analysis/testdata/allinone/services/authn/internal/auth.go:18:6: unreachable func: RunFromTest\n"+
			"analysis/testdata/allinone/services/authn/internal/generated.go:5:6: unreachable func: Generated\n",
		),
		Entry("Tests", func(r *analysis.Runner) {
			r.TestFlag = true
		}, "analysis/testdata/allinone/pkg/http/http.go:17:6: unreachable func: Put\n"+
			"analysis/testdata/allinone/pkg/http/http.go:9:6: unreachable func: Get\n"+
			"analysis/testdata/allinone/pkg/logging/logging.go:12:6: unreachable func: Warn\n",
		),
		Entry("Tags", func(r *analysis.Runner) {
			r.TagsFlag = "rpi"
		}, "analysis/testdata/allinone/pkg/http/http.go:17:6: unreachable func: Put\n"+
			"analysis/testdata/allinone/pkg/logging/logging.go:12:6: unreachable func: Warn\n"+
			"analysis/testdata/allinone/pkg/logging/logging.go:6:6: unreachable func: Debug\n"),
	)

	It("Handles properly multiple modules with filter", func() {
		ctx := context.Background()
		r := analysis.New(stdOut, stdErr, []string{
			"testdata/allinone/services/config/main.go",
			"testdata/cli/main.go",
		})
		r.FilterFlag = "Masterminds/semver"
		Expect(r.Run(ctx)).To(Succeed())

		lines := strings.Split(strings.TrimSpace(stdOut.String()), "\n")
		foundStrictNewVersion, foundMustParse := "", ""
		for _, line := range lines {
			if strings.Contains(line, "StrictNewVersion") {
				foundStrictNewVersion = line
			} else if strings.Contains(line, "MustParse") {
				foundMustParse = line
			}
			Expect(line).To(ContainSubstring("github.com/!masterminds/semver/v3@"))
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
				Path: "github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/cache",
				Funcs: []*analysis.Function{
					{
						Name: "Delete",
						Position: analysis.Position{
							File: "analysis/testdata/allinone/pkg/cache/cache.go",
							Line: 12,
							Col:  6,
						},
					},
				},
			},
			{
				Name: "logging",
				Path: "github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/logging",
				Funcs: []*analysis.Function{
					{
						Name: "Debug",
						Position: analysis.Position{
							File: "analysis/testdata/allinone/pkg/logging/logging.go",
							Line: 6,
							Col:  6,
						},
					},
					{
						Name: "Warn",
						Position: analysis.Position{
							File: "analysis/testdata/allinone/pkg/logging/logging.go",
							Line: 12,
							Col:  6,
						},
					},
				},
			},
			{
				Name: "internal",
				Path: "github.com/arxeiss/deadmono/analysis/testdata/allinone/services/authn/internal",
				Funcs: []*analysis.Function{
					{
						Name: "RunFromTest",
						Position: analysis.Position{
							File: "analysis/testdata/allinone/services/authn/internal/auth.go",
							Line: 18,
							Col:  6,
						},
					},
					{
						Name: "Generated",
						Position: analysis.Position{
							File: "analysis/testdata/allinone/services/authn/internal/generated.go",
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
