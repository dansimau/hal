SHELL := /bin/bash

# golangci-lint is a Go tool pinned in go.mod (tool directive).
.PHONY: lint
lint:
	go tool golangci-lint run

.PHONY: test
test:
	which go-test-coverage || go install github.com/vladopajic/go-test-coverage/v2@latest
	set -o pipefail; go test -race -v ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./... -json | python3 testutil/colourise-go-test-output.py >/dev/null
	go-test-coverage --config=./.testcoverage.yaml
