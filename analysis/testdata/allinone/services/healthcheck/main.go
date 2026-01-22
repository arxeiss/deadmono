package main

import "github.com/arxeiss/deadmono/analysis/testdata/allinone/pkg/http"

func main() {
	http.New()
	http.Get()
	http.Post()
	http.Put()
	http.Delete()
}
