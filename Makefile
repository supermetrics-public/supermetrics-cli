-include .env

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%d)
PKG     := github.com/supermetrics-public/supermetrics-cli/internal/buildcfg
LDFLAGS := -X $(PKG).Version=$(VERSION) \
           -X $(PKG).Commit=$(COMMIT) \
           -X $(PKG).BuildDate=$(DATE) \
           -X $(PKG).OAuthClientID=$(SUPERMETRICS_OAUTH_CLIENT_ID) \
           -X '$(PKG).OAuthScopes=$(SUPERMETRICS_OAUTH_SCOPES)'
ifdef SUPERMETRICS_DOMAIN
LDFLAGS += -X $(PKG).DefaultDomain=$(SUPERMETRICS_DOMAIN)
endif

.PHONY: build build-release run install test test-go test-python test-coverage lint lint-python lint-fix clean generate tools snapshot vet vulncheck tidy-check

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/supermetrics .

build-release:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o bin/supermetrics .

run:
	go run -ldflags "$(LDFLAGS)" . $(ARGS)

install:
	go install -ldflags "$(LDFLAGS)" .

test: test-go test-python

test-go:
	@if command -v gotestsum >/dev/null 2>&1; then \
		gotestsum --format testname ./...; \
	else \
		go test ./...; \
	fi

test-python:
	cd scripts && uv run python3 -m unittest test_generate_commands -v

test-coverage:
	go test -race -coverprofile=coverage.raw.out ./...
	cp coverage.raw.out coverage.out
	while IFS= read -r pattern || [ -n "$$pattern" ]; do \
		grep -v "$$pattern" coverage.out > coverage.out.tmp && mv coverage.out.tmp coverage.out; \
	done < .covignore
	rm coverage.raw.out
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...
	$(MAKE) lint-python

lint-python:
	uv run ruff check scripts/

lint-fix:
	golangci-lint run --fix ./...
	uv run ruff check --fix scripts/

vet:
	go vet ./...

vulncheck:
	govulncheck ./...

tidy-check:
	go mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy"; exit 1)

clean:
	rm -rf bin/ coverage.out coverage.raw.out

generate:
	uv run python3 scripts/generate_commands.py
	goimports -w cmd/generated/

tools:
	go install golang.org/x/tools/cmd/goimports@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install gotest.tools/gotestsum@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(shell cat .golangci-lint-version)
	uv tool install ruff

snapshot:
	goreleaser build --snapshot --clean
