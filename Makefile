.phony: lint lint-fix

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

test:
	go test -v -covermode=count -coverprofile=coverage.out ./...
