GO ?= go

# Integration tests that need a live MySQL are guarded by testing.Short(),
# so every target below runs with -short. See docs/known-issues.md for the
# tests excluded from the race target.
RACE_SKIP ?= TestConfigWatcherDirtyOverrite

.PHONY: all build vet test test-race lint vuln tidy api-snapshot api-check ci

all: ci

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test -short ./...

test-race:
	$(GO) test -short -race -timeout 600s -skip '$(RACE_SKIP)' ./...

lint:
	$(GO) run honnef.co/go/tools/cmd/staticcheck@latest ./...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@latest ./...

tidy:
	$(GO) mod tidy
	git diff --exit-code go.mod go.sum

api-snapshot:
	scripts/api-snapshot.sh

# Fails when the committed snapshot no longer matches the code, forcing API
# changes to surface in the pull request diff.
api-check: api-snapshot
	git diff --exit-code api/

ci: tidy build vet test-race api-check
