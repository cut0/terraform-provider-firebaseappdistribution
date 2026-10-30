default: fmt lint build test

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run

generate:
	go tool tfplugindocs generate --provider-name firebaseappdistribution

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 30m ./...

.PHONY: build install lint generate fmt test testacc
