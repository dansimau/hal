SHELL := /bin/bash

# golangci-lint is a Go tool pinned in golangci-lint.mod. It lives in its own
# modfile so its (many) dependencies and Go version requirement do not leak
# into the module's go.mod.
.PHONY: lint
lint:
	go tool -modfile=golangci-lint.mod golangci-lint run

.PHONY: test
test:
	which go-test-coverage || go install github.com/vladopajic/go-test-coverage/v2@latest
	set -o pipefail; go test -race -v ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./... -json | python3 testutil/colourise-go-test-output.py >/dev/null
	go-test-coverage --config=./.testcoverage.yaml
