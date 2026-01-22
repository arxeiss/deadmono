package internal_test

import (
	"testing"

	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/http"
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/logging"
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/services/authn/internal"
)

func TestRunFromTest(t *testing.T) {
	http.Post()
	logging.Debug()
	internal.RunFromTest()
}
