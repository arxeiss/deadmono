package http

import "github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/logging"

func New() {
	logging.New()
}

func Get() {
	logging.Info()
}

func Post() {
	logging.Info()
}

func Put() {
	logging.Info()
}

func Delete() {
	logging.Info()
}
