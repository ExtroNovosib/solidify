BINARY := solidlint
MODULE := solidlint
COMMAND := ./cmd/solidlint
GO := go
GOFLAGS ?=
LDFLAGS ?=
TEST_VERSION ?= v0.0.0-ci
COVERAGE_FLOOR ?= 80

BUILD_DIR := bin
BUILD := $(BUILD_DIR)/$(BINARY)
VERSION_BUILD := $(BUILD_DIR)/$(BINARY)-version-check
PLUGIN := $(BUILD_DIR)/solidlint.so
PLUGIN_HOST := $(BUILD_DIR)/golangci-lint
PLUGIN_GO_E2E_LOG := $(BUILD_DIR)/plugin-go-e2e.log

GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
GORELEASER ?= goreleaser
BASELINE ?= .solidlint-baseline.json
# Self-enforcement scans only the analyzer implementation so deliberate fixture
# violations remain outside its accepted-debt baseline contract.
SELF_LINT_PKG := ./internal/analyzer/...
# General quality checks cover each buildable first-party production package.
QUALITY_PKG := ./internal/... ./plugin/... ./cmd/...
# The ordinary E2E target discovers every Test* in tests/e2e at run time, then
# excludes only workflows owned by their dedicated plugin/contract targets.
E2E_ORDINARY_EXCLUDED_TESTS := TestCustomGolangCIModulePluginHonorsSelectedChecks|TestGoPluginGateContract|TestCanonicalGateOwnsExpensivePluginBuildOnce
RACE_PKG := ./internal/... ./plugin/... ./cmd/... ./tests/integration

.PHONY: all build plugin run report enforce install test test-unit test-integration test-e2e test-race coverage vet vulncheck fmt fmt-check golangci-lint lint smoke precision cli-e2e plugin-module-e2e plugin-go-e2e plugin-go-e2e-contract test-ownership-contract e2e cache-parity sarif-check schema-check release-consumer-smoke release-snapshot publish-release-test publish version-check check-fast check check-release clean help

all: build

$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

build: $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD) $(COMMAND)

plugin: $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -tags plugin -buildmode=plugin -o $(PLUGIN) ./cmd/solidlint-golangci

run: report

# Report every finding without changing the command's exit status. This is
# intended for local exploration and artifact generation, not policy checks.
report: build
	$(BUILD) -fail=false $(SELF_LINT_PKG)

# Fail only for new findings in the tool's implementation. The baseline keeps
# currently accepted debt visible without letting it block incremental cleanup.
enforce: build
	$(BUILD) -config .solidlint-enforce.yml -baseline $(BASELINE) $(SELF_LINT_PKG)

install:
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" $(COMMAND)

test:
	$(GO) test $(GOFLAGS) ./...

test-unit:
	$(GO) test $(GOFLAGS) ./internal/analyzer ./internal/config ./internal/baseline ./internal/report ./internal/analysisapi ./internal/cli ./plugin/solidlint ./cmd/solidlint -count=1

test-integration:
	$(GO) test $(GOFLAGS) ./tests/integration -count=1

test-e2e:
	@names="$$($(GO) test $(GOFLAGS) ./tests/e2e -list '^Test' | awk '/^Test/ && $$0 !~ /^($(E2E_ORDINARY_EXCLUDED_TESTS))$$/ { printf "%s|", $$0 }' | sed 's/|$$//')"; \
		test -n "$$names" || (echo "no ordinary E2E tests were discovered" >&2; exit 1); \
		$(GO) test $(GOFLAGS) ./tests/e2e -run "^($$names)$$" -count=1

test-race:
	$(GO) test $(GOFLAGS) -race $(RACE_PKG)

coverage: $(BUILD_DIR)
	$(GO) test $(GOFLAGS) -coverpkg=./internal/... -coverprofile=$(BUILD_DIR)/coverage.out ./internal/... ./tests/integration
	@total="$$(go tool cover -func=$(BUILD_DIR)/coverage.out | awk '/^total:/ { gsub("%", "", $$3); print $$3 }')"; \
	awk -v total="$$total" -v floor="$(COVERAGE_FLOOR)" 'BEGIN { if (total + 0 < floor + 0) { printf "analyzer coverage %.1f%% is below %s%%\\n", total, floor; exit 1 } }'

vet:
	$(GO) vet $(QUALITY_PKG)

vulncheck:
	govulncheck ./...

fmt:
	$(GO) fmt ./...

# Uses only find and gofmt so a missing tool cannot turn the check into a
# vacuous pass; gofmt parse errors fail the target as well.
fmt-check:
	@files="$$(find . \( -path './.*' -o -path ./bin -o -path ./dist -o -path ./graphify-out -o -path ./testdata -o -path ./internal/analysisapi/testdata \) -prune -o -name '*.go' -print)"; \
		test -n "$$files" || { echo "fmt-check found no Go files" >&2; exit 1; }; \
		unformatted="$$(gofmt -l $$files)" || exit 1; \
		test -z "$$unformatted" || { echo "gofmt would change:" >&2; echo "$$unformatted" >&2; exit 1; }

golangci-lint:
	$(GOLANGCI_LINT) run $(QUALITY_PKG)

lint: golangci-lint enforce

smoke: build
	$(BUILD) -fail=false testdata/violations
	$(BUILD) testdata/clean

precision:
	$(GO) test ./internal/analyzer -run '^(TestPrecisionCorpus|TestStableEvaluationManifestCoverageAndVerdicts|TestStableEvaluationEdgeStyles)$$' -count=1

cli-e2e: build
	$(BUILD) -profile=stable -format=json -fail=false ./testdata/violations > $(BUILD_DIR)/stable.json
	$(BUILD) -profile=all -format=sarif -fail=false ./testdata/violations > $(BUILD_DIR)/all.sarif
	$(BUILD) -analysis=syntax -fail=false ./testdata/clean

plugin-module-e2e:
	$(GO) test $(GOFLAGS) ./tests/e2e -run '^TestCustomGolangCIModulePluginHonorsSelectedChecks$$' -count=1

plugin-go-e2e: $(BUILD_DIR)
	@if [ "$$(uname -s)" = Linux ]; then \
		CGO_ENABLED=1 $(GO) build $(GOFLAGS) -tags plugin -buildmode=plugin -o $(PLUGIN) ./cmd/solidlint-golangci; \
		CGO_ENABLED=1 GOBIN=$(CURDIR)/$(BUILD_DIR) $(GO) install $(GOFLAGS) github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2; \
		(cd internal/analysisapi/testdata/src/fat && \
			GOCACHE=$(CURDIR)/.cache/go-build $(CURDIR)/$(PLUGIN_HOST) run -c $(CURDIR)/.golangci-go-plugin.yml ./... > $(CURDIR)/$(PLUGIN_GO_E2E_LOG) 2>&1; \
			status=$$?; \
			if [ "$$status" -eq 0 ]; then \
				cat $(CURDIR)/$(PLUGIN_GO_E2E_LOG) >&2; \
				echo "shared plugin host unexpectedly accepted the violation fixture" >&2; \
				exit 1; \
			fi; \
			if [ "$$status" -ne 1 ]; then \
				cat $(CURDIR)/$(PLUGIN_GO_E2E_LOG) >&2; \
				echo "shared plugin host failed with exit $$status; expected violation exit 1" >&2; \
				exit 1; \
			fi); \
		grep -q 'SOLID-I/fat-interface' $(PLUGIN_GO_E2E_LOG); \
	else echo "Go shared plugins are verified on Linux CI"; fi

plugin-go-e2e-contract:
	$(GO) test $(GOFLAGS) ./tests/e2e -run '^TestGoPluginGateContract$$' -count=1

test-ownership-contract:
	$(GO) test $(GOFLAGS) ./tests/e2e -run '^TestCanonicalGateOwnsExpensivePluginBuildOnce$$' -count=1

cache-parity:
	$(GO) test ./internal/analyzer -run 'Test.*Cache' -count=1

e2e: test-e2e plugin-module-e2e plugin-go-e2e-contract plugin-go-e2e

sarif-check:
	$(GO) test ./... -run 'TestSARIF' -count=1

schema-check:
	$(GO) test ./internal/config ./internal/report ./internal/baseline -run 'Test.*(Schema|JSON)' -count=1

release-consumer-smoke:
	@test -n "$(SOLIDLINT_VERSION)" || (echo "SOLIDLINT_VERSION is required (for example, v0.1.0)" >&2; exit 2)
	SOLIDLINT_VERSION=$(SOLIDLINT_VERSION) ./scripts/release-consumer-smoke.sh

release-snapshot:
	@if command -v $(GORELEASER) >/dev/null 2>&1; then $(GORELEASER) release --snapshot --clean; else echo "goreleaser is required for release-snapshot" >&2; exit 1; fi

publish-release-test:
	./scripts/publish-release_test.sh

publish:
	@test -n "$(VERSION)" || (echo "VERSION is required (for example, v0.2.0)" >&2; exit 2)
	./scripts/publish-release.sh $(PUBLISH_FLAGS) $(VERSION)

version-check: $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "-X main.version=$(TEST_VERSION)" -o $(VERSION_BUILD) $(COMMAND)
	@test "$$($(VERSION_BUILD) -version)" = "$(TEST_VERSION)"
	@$(VERSION_BUILD) -format=sarif -fail=false ./testdata/clean > $(BUILD_DIR)/version-check.sarif
	@grep -q '"version":"$(TEST_VERSION)"' $(BUILD_DIR)/version-check.sarif
	@$(VERSION_BUILD) -cache-debug -fail=false ./testdata/clean >/dev/null 2>$(BUILD_DIR)/version-check.stderr
	@grep -q 'version=$(TEST_VERSION)' $(BUILD_DIR)/version-check.stderr

check-fast: fmt-check vet test-unit test-integration lint

check: check-fast test-e2e test-race coverage smoke precision cli-e2e cache-parity sarif-check schema-check plugin-module-e2e plugin-go-e2e-contract test-ownership-contract plugin-go-e2e version-check vulncheck

check-release: check release-snapshot release-consumer-smoke

clean:
	rm -rf $(BUILD_DIR)

help:
	@echo "Targets:"
	@echo "  build   - compile $(BINARY) into $(BUILD)"
	@echo "  plugin  - build the golangci-lint plugin using the documented plugin tag"
	@echo "  run     - alias for report"
	@echo "  report  - print findings for $(SELF_LINT_PKG) without failing"
	@echo "  enforce - fail on new analyzer findings, relative to $(BASELINE)"
	@echo "  install - install $(BINARY) to GOPATH/bin"
	@echo "  test    - run go test ./..."
	@echo "  test-unit - run focused unit/package tests"
	@echo "  test-integration - run cross-package integration tests"
	@echo "  test-e2e - discover and run every ordinary CLI subprocess journey"
	@echo "  test-race - run first-party production and integration packages with the race detector"
	@echo "  test-ownership-contract - verify E2E/plugin workflows have exactly one canonical check owner"
	@echo "  coverage - enforce the analyzer coverage floor"
	@echo "  vet     - run go vet ./..."
	@echo "  vulncheck - scan all packages with govulncheck"
	@echo "  fmt     - run go fmt ./..."
	@echo "  fmt-check - fail when gofmt would change files"
	@echo "  golangci-lint - run golangci-lint on $(QUALITY_PKG)"
	@echo "  lint    - run golangci-lint and enforce new $(BINARY) findings"
	@echo "  smoke   - report deliberate violations and enforce a clean fixture"
	@echo "  precision - run the positive/negative corpus precision gate"
	@echo "  sarif-check - validate representative SARIF output through regression tests"
	@echo "  schema-check - validate CLI JSON output against solidlint-result-v3.schema.json"
	@echo "  release-consumer-smoke - install a published version and exercise its CLI and GolangCI module plugin"
	@echo "  release-snapshot - build release archives, checksums, and SBOMs with GoReleaser"
	@echo "  publish-release-test - verify publishing-script argument validation"
	@echo "  publish - qualify and push VERSION as an immutable release tag"
	@echo "  version-check - verify linker-injected version reporting surfaces"
	@echo "  check-fast - run the short formatting, vet, unit, integration, and lint loop"
	@echo "  check   - run every local reliability gate once"
	@echo "  check-release - run check plus release snapshot and external-consumer smoke"
	@echo "  clean   - remove $(BUILD_DIR)/"
	@echo "  help    - show this help"
