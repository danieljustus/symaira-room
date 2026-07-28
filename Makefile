VERSION ?= dev
LDFLAGS := -X github.com/danieljustus/symaira-room/internal/version.Version=$(VERSION)

.PHONY: build test test-race lint fmt-check clean

CGO_ENABLED ?= 0

build:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS)" -o bin/symroom ./cmd/symroom

test:
	go test -v ./...

test-race:
	go test -race -v ./...

lint:
	gofmt -s -w .
	go vet ./...

fmt-check:
	@test -z $$(gofmt -l .) || (echo "Unformatted files found:" && gofmt -l . && exit 1)

clean:
	rm -rf bin/
