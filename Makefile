.PHONY: lint
lint:
	which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.62.2
	golangci-lint run

.PHONY: test
test:
	go test -v ./...
