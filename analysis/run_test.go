package analysis_test

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"

	"github.com/arxeiss/deadmono/analysis"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		r := analysis.New(stdOut, stdErr, []string{"testdata/allinone/services/authn/main.go", "/home"})
		err := r.Run(ctx)
		Expect(err).To(MatchError(
			"different modules in one run are not supported: github.com/arxeiss/deadmono/ != command-line-arguments/"))
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
				"Detected 3 dependencies\n" +
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
})
