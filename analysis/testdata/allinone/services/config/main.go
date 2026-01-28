package main

import (
	"github.com/Masterminds/semver/v3"

	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/cache"
	"github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/logging"
)

func main() {
	_, _ = semver.StrictNewVersion("v1.2.3")

	cache.New()
	cache.Get()
	cache.Set()

	logging.Error()
}
