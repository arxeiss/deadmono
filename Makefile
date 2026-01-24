.phony: lint lint-fix

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

test:
	go test -v -covermode=count -coverprofile=coverage.tmp.out ./...
	cat coverage.tmp.out | grep -v "main.go" > coverage.out
	rm coverage.tmp.out
