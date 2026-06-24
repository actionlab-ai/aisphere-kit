.PHONY: fmt vet test build lint coverage ci tidy

tidy:
	go mod tidy

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

test:
	go test ./...

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

build:
	go build ./...

lint:
	golangci-lint run ./...

ci: tidy fmt vet test build
