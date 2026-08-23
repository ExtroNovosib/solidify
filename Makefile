BINARY := solidlint
MODULE := solidlint
GO := go
GOFLAGS ?=
LDFLAGS ?=
TEST_VERSION ?= v0.0.0-ci
COVERAGE_FLOOR ?= 80

BUILD_DIR := bin
BUILD := $(BUILD_DIR)/$(BINARY)
VERSION_BUILD := $(BUILD_DIR)/$(BINARY)-version-check

GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
GORELEASER ?= goreleaser
BASELINE ?= .solidlint-baseline.json
# Self-check scope: lint the analyzer implementation, not fixture corpora.
LINT_PKG := ./internal/analyzer/...

.PHONY: all build plugin run report enforce install test test-unit test-integration test-race coverage vet vulncheck fmt fmt-check golangci-lint lint smoke precision cli-e2e plugin-module-e2e plugin-go-e2e e2e cache-parity sarif-check schema-check release-snapshot version-check check clean help

all: build

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

build: $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD) .

plugin: $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -tags plugin -buildmode=plugin -o $(BUILD_DIR)/solidlint.so ./cmd/solidlint-golangci

run: report

# Report every finding without changing the command's exit status. This is
# intended for local exploration and artifact generation, not policy checks.
report: build
	$(BUILD) -fail=false $(LINT_PKG)

# Fail only for new findings in the tool's implementation. The baseline keeps
# currently accepted debt visible without letting it block incremental cleanup.
enforce: build
	$(BUILD) -config .solidlint-enforce.yml -baseline $(BASELINE) $(LINT_PKG)

install:
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" .

test:
	$(GO) test $(GOFLAGS) ./...

test-unit:
	$(GO) test $(GOFLAGS) ./internal/analyzer ./internal/config ./internal/baseline ./internal/report ./internal/analysisapi ./plugin/solidlint -count=1

test-integration:
	$(GO) test $(GOFLAGS) ./tests/integration -count=1

test-race:
	$(GO) test $(GOFLAGS) -race ./...

coverage: $(BUILD_DIR)
	$(GO) test $(GOFLAGS) -coverpkg=./internal/... -coverprofile=$(BUILD_DIR)/coverage.out ./internal/... ./tests/integration
	@total="$$(go tool cover -func=$(BUILD_DIR)/coverage.out | awk '/^total:/ { gsub("%", "", $$3); print $$3 }')"; \
	awk -v total="$$total" -v floor="$(COVERAGE_FLOOR)" 'BEGIN { if (total + 0 < floor + 0) { printf "analyzer coverage %.1f%% is below %s%%\\n", total, floor; exit 1 } }'

vet:
	$(GO) vet $(LINT_PKG)

vulncheck:
	govulncheck ./...

fmt:
	$(GO) fmt ./...

fmt-check:
	@test -z "$$(rg --files -g '*.go' -g '!graphify-out/**' -g '!.cache/**' -g '!testdata/**' -g '!internal/analysisapi/testdata/**' | xargs gofmt -l)"

golangci-lint:
	$(GOLANGCI_LINT) run $(LINT_PKG)

lint: golangci-lint enforce

smoke: build
	$(BUILD) -fail=false testdata/violations
	$(BUILD) testdata/clean

precision:
	$(GO) test ./internal/analyzer -run TestPrecisionCorpus -count=1

cli-e2e: build
	$(BUILD) -profile=stable -format=json -fail=false ./testdata/violations > $(BUILD_DIR)/stable.json
	$(BUILD) -profile=all -format=sarif -fail=false ./testdata/violations > $(BUILD_DIR)/all.sarif
	$(BUILD) -analysis=syntax -fail=false ./testdata/clean

plugin-module-e2e:
	$(GO) test $(GOFLAGS) ./plugin/solidlint ./internal/analysisapi -count=1
	$(GOLANGCI_LINT) custom -v
	@cd internal/analysisapi/testdata/src/fat && ! GOCACHE=$(CURDIR)/.cache/go-build ../../../../../bin/solidlint-golangci run -c ../../../../../.golangci-plugin.yml ./... > ../../../../../$(BUILD_DIR)/plugin-module-e2e.log 2>&1
	@grep -q 'SOLID-I/fat-interface' $(BUILD_DIR)/plugin-module-e2e.log

plugin-go-e2e: $(BUILD_DIR)
	@if [ "$$(uname -s)" = Linux ]; then \
		CGO_ENABLED=1 $(GO) build $(GOFLAGS) -tags plugin -buildmode=plugin -o $(BUILD_DIR)/solidlint.so ./cmd/solidlint-golangci; \
		(cd internal/analysisapi/testdata/src/fat && ! GOCACHE=$(CURDIR)/.cache/go-build ../../../../../bin/solidlint-golangci run -c ../../../../../.golangci-go-plugin.yml ./... > ../../../../../$(BUILD_DIR)/plugin-go-e2e.log 2>&1); \
		grep -q 'SOLID-I/fat-interface' $(BUILD_DIR)/plugin-go-e2e.log; \
	else echo "Go shared plugins are verified on Linux CI"; fi

cache-parity:
	$(GO) test ./internal/analyzer -run 'TestPackageCache|Test.*Cache.*Parity' -count=1

e2e: cli-e2e plugin-module-e2e plugin-go-e2e release-snapshot

sarif-check:
	$(GO) test ./... -run 'TestSARIF' -count=1

schema-check:
	$(GO) test ./... -run 'Test(IssuesJSON|EncodeIssuesJSON)' -count=1

release-snapshot:
	@if command -v $(GORELEASER) >/dev/null 2>&1; then $(GORELEASER) release --snapshot --clean; else echo "goreleaser is required for release-snapshot" >&2; exit 1; fi

version-check: $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "-X main.version=$(TEST_VERSION)" -o $(VERSION_BUILD) .
	@test "$$($(VERSION_BUILD) -version)" = "$(TEST_VERSION)"
	@$(VERSION_BUILD) -format=sarif -fail=false ./testdata/clean > $(BUILD_DIR)/version-check.sarif
	@grep -q '"version":"$(TEST_VERSION)"' $(BUILD_DIR)/version-check.sarif
	@$(VERSION_BUILD) -cache-debug -fail=false ./testdata/clean >/dev/null 2>$(BUILD_DIR)/version-check.stderr
	@grep -q 'version=$(TEST_VERSION)' $(BUILD_DIR)/version-check.stderr

check: fmt-check vet test-unit test-integration test test-race coverage lint smoke precision cli-e2e cache-parity sarif-check schema-check plugin-module-e2e plugin-go-e2e version-check vulncheck

clean:
	rm -rf $(BUILD_DIR)

help:
	@echo "Targets:"
	@echo "  build   - compile $(BINARY) into $(BUILD)"
	@echo "  plugin  - build the golangci-lint plugin using the documented plugin tag"
	@echo "  run     - alias for report"
	@echo "  report  - print findings for $(LINT_PKG) without failing"
	@echo "  enforce - fail on new analyzer findings, relative to $(BASELINE)"
	@echo "  install - install $(BINARY) to GOPATH/bin"
	@echo "  test    - run go test ./..."
	@echo "  test-race - run the full test suite with the race detector"
	@echo "  coverage - enforce the analyzer coverage floor"
	@echo "  vet     - run go vet ./..."
	@echo "  vulncheck - scan all packages with govulncheck"
	@echo "  fmt     - run go fmt ./..."
	@echo "  fmt-check - fail when gofmt would change files"
	@echo "  golangci-lint - run golangci-lint on $(LINT_PKG)"
	@echo "  lint    - run golangci-lint and enforce new $(BINARY) findings"
	@echo "  smoke   - report deliberate violations and enforce a clean fixture"
	@echo "  precision - run the positive/negative corpus precision gate"
	@echo "  sarif-check - validate representative SARIF output through regression tests"
	@echo "  schema-check - validate CLI JSON output against solidlint-result-v3.schema.json"
	@echo "  release-snapshot - build release archives, checksums, and SBOMs with GoReleaser"
	@echo "  version-check - verify linker-injected version reporting surfaces"
	@echo "  check   - run every local reliability gate"
	@echo "  clean   - remove $(BUILD_DIR)/"
	@echo "  help    - show this help"
