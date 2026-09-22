package internal_test

import (
	"testing"

	"github.com/arxeiss/deadmono/sample/allinone/pkg/http"
	"github.com/arxeiss/deadmono/sample/allinone/pkg/logging"
	"github.com/arxeiss/deadmono/sample/allinone/services/authn/internal"
)

func TestRunFromTest(t *testing.T) {
	http.Post()
	logging.Debug()
	internal.RunFromTest()
}
