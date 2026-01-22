package main

import (
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/cache"
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/logging"
)

func main() {
	cache.New()
	cache.Get()
	cache.Set()

	logging.Error()
}
