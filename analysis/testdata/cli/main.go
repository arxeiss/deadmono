package main

import (
	"github.com/Masterminds/semver/v3"
)

func main() {
	_ = semver.MustParse("v1.2.3")
}
