VERSION ?= dev
LDFLAGS := -X github.com/danieljustus/symaira-room/internal/version.Version=$(VERSION)

.PHONY: build test test-race lint fmt-check release-check release-dry-run clean

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

release-check:
	@test -f .goreleaser.yaml
	@command -v goreleaser >/dev/null || (echo "goreleaser is required for release-check" >&2 && exit 1)
	goreleaser check

release-dry-run:
	@test -f .goreleaser.yaml
	@command -v goreleaser >/dev/null || (echo "goreleaser is required for release-dry-run" >&2 && exit 1)
	goreleaser release --snapshot --clean --skip=sign,sbom,publish

clean:
	rm -rf bin/
