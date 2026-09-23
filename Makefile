ENGINE ?= docker
GO_IMAGE ?= golang:1.26-bookworm
GO_CONTAINER = $(ENGINE) run --rm --user $(shell id -u):$(shell id -g) \
	-v "$(CURDIR):/workspace:z" -w /workspace \
	-e GOCACHE=/workspace/.cache/build -e GOPATH=/workspace/.cache/go $(GO_IMAGE)

.PHONY: build test vet fmt tidy image run

build:
	$(GO_CONTAINER) go build -trimpath -o gourl ./cmd/gourl

test:
	$(GO_CONTAINER) go test -race ./...

vet:
	$(GO_CONTAINER) go vet ./...

fmt:
	$(GO_CONTAINER) go fmt ./...

tidy:
	$(GO_CONTAINER) go mod tidy

image:
	$(ENGINE) build -t gourl .

run:
	$(ENGINE) run --rm -it -e TERM -v gourl-data:/data gourl