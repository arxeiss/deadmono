//go:build !rpi

package internal

import (
	"github.com/arxeiss/deadmono/sample/allinone/pkg/http"
	"github.com/arxeiss/deadmono/sample/allinone/pkg/logging"
)

func Run() {
	http.New()

	http.Delete()

	logging.Error()
}

func RunFromTest() {
}
