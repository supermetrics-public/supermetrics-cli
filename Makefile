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

# Dev tools are pinned in mise.toml; `mise exec --` makes the targets work
# without shell activation. `go` is left bare — GOTOOLCHAIN=auto and the go.mod
# directive pin it, and goreleaser-action invokes `go` outside any wrapper.
MISE := mise exec --

.PHONY: build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/supermetrics ./cmd/supermetrics

.PHONY: build-release
build-release:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o bin/supermetrics ./cmd/supermetrics

.PHONY: run
run:
	go run -ldflags "$(LDFLAGS)" ./cmd/supermetrics $(ARGS)

.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/supermetrics

.PHONY: test
test: test-go test-python

.PHONY: test-go
test-go:
	$(MISE) gotestsum --format testname ./...

.PHONY: test-python
test-python:
	cd scripts && $(MISE) uv run python3 -m unittest test_generate_commands -v

.PHONY: test-coverage
test-coverage:
	go test -race -coverprofile=coverage.raw.out ./...
	cp coverage.raw.out coverage.out
	while IFS= read -r pattern || [ -n "$$pattern" ]; do \
		grep -v "$$pattern" coverage.out > coverage.out.tmp && mv coverage.out.tmp coverage.out; \
	done < .covignore
	rm coverage.raw.out
	go tool cover -func=coverage.out

.PHONY: lint
lint:
	$(MISE) golangci-lint run ./...
	$(MAKE) lint-python

.PHONY: lint-python
lint-python:
	$(MISE) ruff check scripts/

.PHONY: lint-fix
lint-fix:
	$(MISE) golangci-lint run --fix ./...
	$(MISE) ruff check --fix scripts/

.PHONY: modernize
modernize:
	go fix ./...

.PHONY: modernize-check
modernize-check:
	@out=$$(go fix -diff ./...); \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		echo "modernizations available; run 'make modernize'"; \
		exit 1; \
	fi

.PHONY: vet
vet:
	go vet ./...

.PHONY: vulncheck
vulncheck:
	$(MISE) govulncheck ./...

.PHONY: tidy-check
tidy-check:
	go mod tidy
	@git diff --exit-code go.mod go.sum || (echo "go.mod/go.sum not tidy"; exit 1)

.PHONY: clean
clean:
	rm -rf bin/ coverage.out coverage.raw.out

.PHONY: generate
generate:
	$(MISE) uv run python3 scripts/generate_commands.py
	$(MISE) goimports -w cmd/generated/

.PHONY: tools
tools:
	mise install

.PHONY: snapshot
snapshot:
	goreleaser build --snapshot --clean
