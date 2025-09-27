
.PHONY: lint
lint:
	which golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.62.2
	golangci-lint run

.PHONY: test
test:
	which go-test-coverage || go install github.com/vladopajic/go-test-coverage/v2@latest
	go test -v ./... -coverprofile=./cover.out -covermode=atomic -coverpkg=./... -json | python3 testutil/colourise-go-test-output.py
	go-test-coverage --config=./.testcoverage.yaml
