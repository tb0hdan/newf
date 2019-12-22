all: lint slow-lint build

LINTS = lint-main.go

slow-lint: $(LINTS)

$(LINTS):
	@golint -set_exit_status=1 $(shell echo $@|awk -F'lint-' '{print $$2}')

build:
	@go mod why
	@go build -a -trimpath -tags netgo -installsuffix netgo -v -x -ldflags "-s -w" -o reverseproxy *.go

lint:
	@golangci-lint run --enable-all

deps:
	@go get -u golang.org/x/lint/golint
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.21.0
