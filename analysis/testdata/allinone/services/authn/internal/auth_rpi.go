//go:build rpi

package internal

import (
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/http"
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/logging"
)

func Run() {
	http.New()
	http.Get()
	http.Post()
	http.Delete()

	logging.Error()
}
